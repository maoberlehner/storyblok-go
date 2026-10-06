package storyblok

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	initialRequestRate = 25
	maxCachedRate      = 1000
	minRequestRate     = 1
	cacheHitsToGrow    = 10
)

var errRateLimitWait = errors.New("storyblok: rate-limit queue exceeded one second")

type adaptiveRateLimiter struct {
	mu         sync.Mutex
	limiter    *rate.Limiter
	uncached   *rate.Limiter
	cacheHits  int
	nextGrowth time.Time
	queue      chan struct{}
	changed    chan struct{}
}

func newAdaptiveRateLimiter() *adaptiveRateLimiter {
	return &adaptiveRateLimiter{
		limiter:    rate.NewLimiter(initialRequestRate, 1),
		uncached:   rate.NewLimiter(initialRequestRate, 1),
		nextGrowth: time.Now().Add(time.Second),
		queue:      make(chan struct{}, 1),
		changed:    make(chan struct{}),
	}
}

func isPublishedStory(request *http.Request) bool {
	return strings.Contains(request.URL.Path, "/stories/") && request.URL.Query().Get("version") == string(Published)
}

func isCacheEligible(request *http.Request) bool {
	return isPublishedStory(request) && request.URL.Query().Get("cv") != ""
}

func (l *adaptiveRateLimiter) wait(request *http.Request) error {
	ctx, cancel := context.WithTimeout(request.Context(), time.Second)
	defer cancel()
	// Only the first waiter reserves a future permit. Other waiters stay in
	// the queue, so a rate reduction cannot leave a batch of old-rate permits.
	select {
	case l.queue <- struct{}{}:
		defer func() { <-l.queue }()
	case <-ctx.Done():
		return rateWaitError(request)
	}
	// Drafts, space metadata, and cv discovery cannot borrow the CDN allowance.
	if !isCacheEligible(request) {
		if err := l.uncached.Wait(ctx); err != nil {
			return rateWaitError(request)
		}
	}
	for {
		l.mu.Lock()
		reservation := l.limiter.Reserve()
		changed := l.changed
		l.mu.Unlock()
		if delay := reservation.Delay(); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				reservation.Cancel()
				return rateWaitError(request)
			case <-changed:
				timer.Stop()
				reservation.Cancel()
				continue
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			reservation.Cancel()
			return rateWaitError(request)
		}
		select {
		case <-changed:
			reservation.Cancel()
			continue
		default:
			return nil
		}
	}
}

func rateWaitError(request *http.Request) error {
	if err := request.Context().Err(); err != nil {
		return err
	}
	return errRateLimitWait
}

// setRate is called with mu held and wakes the current waiter to re-reserve.
func (l *adaptiveRateLimiter) setRate(now time.Time, limit rate.Limit) {
	if limit == l.limiter.Limit() {
		return
	}
	l.limiter.SetLimitAt(now, limit)
	close(l.changed)
	l.changed = make(chan struct{})
}

func (l *adaptiveRateLimiter) observe(request *http.Request, response *http.Response) {
	if response == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	current := l.limiter.Limit()
	if response.StatusCode == http.StatusTooManyRequests {
		// Drop out of the CDN tier immediately; repeated 429s can go below 25/s.
		l.setRate(now, max(minRequestRate, min(initialRequestRate, current/2)))
		l.cacheHits = 0
		l.nextGrowth = now.Add(5 * time.Second)
		return
	}
	if !isPublishedStory(request) {
		return
	}
	cache := strings.TrimSpace(response.Header.Get("X-Cache"))
	hit := strings.EqualFold(cache, "Hit from cloudfront") || strings.EqualFold(cache, "HIT")
	if response.StatusCode != http.StatusOK || !isCacheEligible(request) || !hit {
		l.setRate(now, min(initialRequestRate, current))
		l.cacheHits = 0
		if earliest := now.Add(time.Second); earliest.After(l.nextGrowth) {
			l.nextGrowth = earliest
		}
		return
	}
	l.cacheHits++
	if l.cacheHits >= cacheHitsToGrow && !now.Before(l.nextGrowth) {
		l.setRate(now, min(maxCachedRate, current*2))
		l.cacheHits = 0
		l.nextGrowth = now.Add(time.Second)
	}
}

func (l *adaptiveRateLimiter) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.setRate(time.Now(), min(initialRequestRate, l.limiter.Limit()))
	l.cacheHits = 0
	if earliest := time.Now().Add(time.Second); earliest.After(l.nextGrowth) {
		l.nextGrowth = earliest
	}
}
