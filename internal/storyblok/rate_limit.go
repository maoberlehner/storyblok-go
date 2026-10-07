package storyblok

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	baseRequestRate  rate.Limit = 25
	maxCachedRate    rate.Limit = 1000
	minRequestRate   rate.Limit = 1
	cacheHitsToGrow             = 10
	growthInterval              = time.Second
	throttleCooldown            = 5 * time.Second
	maxPermitWait               = 2 * time.Second
)

const (
	cloudFrontCacheHit = "Hit from cloudfront"
	genericCacheHit    = "HIT"
)

// adaptiveRateLimiter paces requests in two tiers. Published story requests
// with a cv can be served from the CDN cache, so their tier grows past the
// base rate while responses are cache hits. Drafts, space metadata, and cv
// discovery stay at or below the base rate in a tier of their own.
type adaptiveRateLimiter struct {
	mu       sync.Mutex
	cached   *pacingTier
	uncached *pacingTier
}

type pacingTier struct {
	limiter    *rate.Limiter
	queue      chan struct{}
	changed    chan struct{}
	cacheHits  int
	nextGrowth time.Time
	nextCut    time.Time
}

func newAdaptiveRateLimiter() *adaptiveRateLimiter {
	return &adaptiveRateLimiter{cached: newPacingTier(), uncached: newPacingTier()}
}

func newPacingTier() *pacingTier {
	return &pacingTier{
		limiter:    rate.NewLimiter(baseRequestRate, burstFor(baseRequestRate)),
		queue:      make(chan struct{}, 1),
		changed:    make(chan struct{}),
		nextGrowth: time.Now().Add(growthInterval),
	}
}

// burstFor allows up to one second of the base rate at once, but never more
// than one second of a throttled rate.
func burstFor(limit rate.Limit) int {
	return int(max(1, min(limit, baseRequestRate)))
}

func isPublishedStory(request *http.Request) bool {
	return strings.Contains(request.URL.Path, "/stories/") && request.URL.Query().Get("version") == string(Published)
}

func isCacheEligible(request *http.Request) bool {
	return isPublishedStory(request) && request.URL.Query().Get("cv") != ""
}

func isCacheHit(response *http.Response) bool {
	cache := strings.TrimSpace(response.Header.Get("X-Cache"))
	return strings.EqualFold(cache, cloudFrontCacheHit) || strings.EqualFold(cache, genericCacheHit)
}

func (l *adaptiveRateLimiter) tierFor(request *http.Request) *pacingTier {
	if isCacheEligible(request) {
		return l.cached
	}
	return l.uncached
}

func (l *adaptiveRateLimiter) wait(request *http.Request) error {
	tier := l.tierFor(request)
	ctx, cancel := context.WithTimeout(request.Context(), maxPermitWait)
	defer cancel()
	// Only the first waiter holds a reservation, so a rate change cannot leave
	// a batch of permits reserved at the old rate.
	select {
	case tier.queue <- struct{}{}:
		defer func() { <-tier.queue }()
	case <-ctx.Done():
		return rateWaitError(request)
	}
	for {
		l.mu.Lock()
		reservation := tier.limiter.Reserve()
		changed := tier.changed
		l.mu.Unlock()
		delay := reservation.Delay()
		if delay == 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			return nil
		case <-changed:
			timer.Stop()
			reservation.Cancel()
		case <-ctx.Done():
			timer.Stop()
			reservation.Cancel()
			return rateWaitError(request)
		}
	}
}

func rateWaitError(request *http.Request) error {
	if err := request.Context().Err(); err != nil {
		return err
	}
	return ErrRateLimited
}

// observe adjusts the tier of the request that produced response. After a
// redirect, that is the final request, not the one the caller sent.
func (l *adaptiveRateLimiter) observe(request *http.Request, response *http.Response) {
	if response == nil {
		return
	}
	if response.Request != nil {
		request = response.Request
	}
	tier := l.tierFor(request)
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		tier.throttle(now)
	case response.StatusCode == http.StatusOK:
		tier.succeed(now, tier == l.cached && isCacheHit(response))
	}
}

// capCachedRate returns the cached tier to the base rate, for example when a
// new cv makes every cached response stale.
func (l *adaptiveRateLimiter) capCachedRate() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cached.capAtBase(time.Now())
}

// The pacingTier methods below must be called with adaptiveRateLimiter.mu held.

func (t *pacingTier) throttle(now time.Time) {
	current := t.limiter.Limit()
	limit := min(baseRequestRate, current)
	// Requests sent before the first 429 arrive as a burst of 429s; halve once
	// per burst rather than once per response.
	if !now.Before(t.nextCut) {
		limit = max(minRequestRate, min(baseRequestRate, current/2))
		t.nextCut = now.Add(growthInterval)
	}
	t.setRate(now, limit)
	t.cacheHits = 0
	t.nextGrowth = now.Add(throttleCooldown)
}

func (t *pacingTier) succeed(now time.Time, hit bool) {
	if !hit {
		t.cacheHits = 0
		if t.limiter.Limit() > baseRequestRate {
			t.capAtBase(now)
			return
		}
	} else {
		t.cacheHits++
	}
	if now.Before(t.nextGrowth) {
		return
	}
	current := t.limiter.Limit()
	switch {
	case current < baseRequestRate:
		t.setRate(now, min(baseRequestRate, current*2))
	case hit && t.cacheHits >= cacheHitsToGrow:
		t.setRate(now, min(maxCachedRate, current*2))
		t.cacheHits = 0
	default:
		return
	}
	t.nextGrowth = now.Add(growthInterval)
}

func (t *pacingTier) capAtBase(now time.Time) {
	t.setRate(now, min(baseRequestRate, t.limiter.Limit()))
	t.cacheHits = 0
	if earliest := now.Add(growthInterval); earliest.After(t.nextGrowth) {
		t.nextGrowth = earliest
	}
}

// setRate wakes the queued waiter so it re-reserves at the new rate.
func (t *pacingTier) setRate(now time.Time, limit rate.Limit) {
	if limit == t.limiter.Limit() {
		return
	}
	t.limiter.SetLimitAt(now, limit)
	t.limiter.SetBurstAt(now, burstFor(limit))
	close(t.changed)
	t.changed = make(chan struct{})
}
