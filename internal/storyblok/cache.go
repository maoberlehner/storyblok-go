package storyblok

import "time"

const cacheVersionRefreshInterval = 30 * time.Second

func (c *Client) currentCacheVersion() int64 {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	// An old cv can stay cached indefinitely. Periodically discover the latest
	// cv through a story response rather than substituting space.version (which
	// can differ from cv for tokens with a TTL).
	if !time.Now().Before(c.nextCVRefresh) {
		return 0
	}
	return c.cacheVersion
}

func (c *Client) updateCacheVersion(cv int64, refreshed bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	// Concurrent responses may arrive out of order; never regress the version.
	if cv <= 0 || cv < c.cacheVersion {
		return
	}
	if cv > c.cacheVersion {
		c.cacheVersion = cv
		c.pacing.reset()
		refreshed = true
	}
	if refreshed {
		c.nextCVRefresh = time.Now().Add(cacheVersionRefreshInterval)
	}
}
