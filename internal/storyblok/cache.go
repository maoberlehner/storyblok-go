package storyblok

import (
	"context"
	"sync"
	"time"
)

// An old cv can stay cached indefinitely, so the client periodically sends a
// published request without cv to learn the current one. space.version is no
// substitute: it differs from cv for tokens with a minimum cache TTL.
const cacheVersionRefreshInterval = 30 * time.Second

type cacheVersionTracker struct {
	mu          sync.Mutex
	version     int64
	nextRefresh time.Time
	// discovery is closed when the in-flight discovery request finishes.
	discovery chan struct{}
}

// next returns the cv for a published request. When discover is true, the
// caller sends no cv and must report the outcome with record.
func (c *cacheVersionTracker) next(ctx context.Context) (cv int64, discover bool, err error) {
	for {
		c.mu.Lock()
		if c.discovery == nil && (c.version == 0 || !time.Now().Before(c.nextRefresh)) {
			c.discovery = make(chan struct{})
			c.mu.Unlock()
			return 0, true, nil
		}
		version, discovery := c.version, c.discovery
		c.mu.Unlock()
		if version > 0 {
			return version, false, nil
		}
		// No cv yet: wait for the first discovery instead of sending more
		// requests without one.
		select {
		case <-discovery:
		case <-ctx.Done():
			return 0, false, ctx.Err()
		}
	}
}

// current returns the known cv and whether published requests still use it
// without discovering a newer one first.
func (c *cacheVersionTracker) current() (cv int64, confirmed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.version, c.version > 0 && time.Now().Before(c.nextRefresh)
}

// record stores the cv a published response reported, or 0 if the request
// failed, and reports whether it is newer than the known version.
func (c *cacheVersionTracker) record(cv int64, discovered bool) (advanced bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if discovered {
		close(c.discovery)
		c.discovery = nil
		if cv > 0 {
			c.nextRefresh = time.Now().Add(cacheVersionRefreshInterval)
		}
	}
	// Concurrent responses may arrive out of order.
	if cv <= c.version {
		return false
	}
	c.version = cv
	return true
}
