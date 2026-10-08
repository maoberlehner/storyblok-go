package storyblok

import (
	"net/http"

	"storyblok-go-website/internal/lru"
)

// responseCacheBytes bounds the memory published API responses take up.
const responseCacheBytes = 64 << 20

type cachedResponse struct {
	header http.Header
	body   []byte
}

// newResponseCache keeps published API responses by request URL. URLs carry
// the cv, so an entry never goes stale: content published later has a new cv
// and misses. Entries of older cvs leave as the least recently used.
func newResponseCache(maxBytes int) *lru.Cache[string, cachedResponse] {
	return lru.New(maxBytes, func(url string, res cachedResponse) int { return len(url) + len(res.body) })
}
