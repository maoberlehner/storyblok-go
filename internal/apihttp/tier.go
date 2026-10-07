package apihttp

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	minRequestRate   rate.Limit = 1
	cacheHitsToGrow             = 10
	growthInterval              = time.Second
	throttleCooldown            = 5 * time.Second
)

// ErrRateLimited reports that a request could not get a permit from the
// client's rate limiter in time.
var ErrRateLimited = errors.New("rate-limit wait exceeded")

// TierConfig describes one request budget.
type TierConfig struct {
	// Base is the starting rate, and the rate the tier recovers to after
	// throttling.
	Base rate.Limit
	// Ceiling above Base lets the tier double its rate while responses are
	// cache hits.
	Ceiling rate.Limit
	// MaxBurst caps how many requests start at once after an idle period.
	MaxBurst int
	// MaxWait fails a request with ErrRateLimited when it cannot get a
	// permit in time. Zero waits as long as the request's context allows.
	MaxWait time.Duration
}

// Tier paces one request budget. A 429 halves its rate, at most once per
// second and down to 1/s, and pauses growth for five seconds. Successful
// responses then double it back to Base, at most once per second.
type Tier struct {
	config     TierConfig
	mu         sync.Mutex
	limiter    *rate.Limiter
	queue      chan struct{}
	changed    chan struct{}
	cacheHits  int
	nextGrowth time.Time
	nextCut    time.Time
}

func NewTier(config TierConfig) *Tier {
	config.Ceiling = max(config.Ceiling, config.Base)
	t := &Tier{
		config:     config,
		queue:      make(chan struct{}, 1),
		changed:    make(chan struct{}),
		nextGrowth: time.Now().Add(growthInterval),
	}
	t.limiter = rate.NewLimiter(config.Base, t.burstFor(config.Base))
	return t
}

// burstFor never allows more than one second of a throttled rate at once.
func (t *Tier) burstFor(limit rate.Limit) int {
	return int(max(1, min(limit, rate.Limit(t.config.MaxBurst))))
}

// Wait implements Pacer for clients that pace all requests in one tier.
func (t *Tier) Wait(request *http.Request) error { return t.Acquire(request.Context()) }

// Observe implements Pacer for responses that are never cached.
func (t *Tier) Observe(response *http.Response) { t.Record(response, false) }

// Acquire blocks until the tier grants a permit.
func (t *Tier) Acquire(ctx context.Context) error {
	parent := ctx
	if t.config.MaxWait > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.config.MaxWait)
		defer cancel()
	}
	waitError := func() error {
		if err := parent.Err(); err != nil {
			return err
		}
		return ErrRateLimited
	}
	// Only the first waiter holds a reservation, so a rate change cannot leave
	// a batch of permits reserved at the old rate.
	select {
	case t.queue <- struct{}{}:
		defer func() { <-t.queue }()
	case <-ctx.Done():
		return waitError()
	}
	for {
		t.mu.Lock()
		reservation := t.limiter.Reserve()
		changed := t.changed
		t.mu.Unlock()
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
			return waitError()
		}
	}
}

// Record adjusts the rate to a response. Only 2xx and 429 responses carry a
// signal; other errors say nothing about the rate limit.
func (t *Tier) Record(response *http.Response, cacheHit bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		t.throttle(now)
	case response.StatusCode >= 200 && response.StatusCode < 300:
		t.succeed(now, cacheHit)
	}
}

// CapAtBase returns the rate to Base, for example when cached responses have
// become stale.
func (t *Tier) CapAtBase() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.capAtBase(time.Now())
}

func (t *Tier) throttle(now time.Time) {
	current := t.limiter.Limit()
	limit := min(t.config.Base, current)
	// Requests sent before the first 429 arrive as a burst of 429s; halve once
	// per burst rather than once per response.
	if !now.Before(t.nextCut) {
		limit = max(minRequestRate, min(t.config.Base, current/2))
		t.nextCut = now.Add(growthInterval)
	}
	t.setRate(now, limit)
	t.cacheHits = 0
	t.nextGrowth = now.Add(throttleCooldown)
}

func (t *Tier) succeed(now time.Time, cacheHit bool) {
	if !cacheHit {
		t.cacheHits = 0
		if t.limiter.Limit() > t.config.Base {
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
	case current < t.config.Base:
		t.setRate(now, min(t.config.Base, current*2))
	case cacheHit && t.cacheHits >= cacheHitsToGrow && current < t.config.Ceiling:
		t.setRate(now, min(t.config.Ceiling, current*2))
		t.cacheHits = 0
	default:
		return
	}
	t.nextGrowth = now.Add(growthInterval)
}

func (t *Tier) capAtBase(now time.Time) {
	t.setRate(now, min(t.config.Base, t.limiter.Limit()))
	t.cacheHits = 0
	if earliest := now.Add(growthInterval); earliest.After(t.nextGrowth) {
		t.nextGrowth = earliest
	}
}

// setRate must be called with t.mu held. It wakes the queued waiter so it
// re-reserves at the new rate.
func (t *Tier) setRate(now time.Time, limit rate.Limit) {
	if limit == t.limiter.Limit() {
		return
	}
	t.limiter.SetLimitAt(now, limit)
	t.limiter.SetBurstAt(now, t.burstFor(limit))
	close(t.changed)
	t.changed = make(chan struct{})
}
