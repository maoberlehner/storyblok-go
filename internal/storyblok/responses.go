package storyblok

import (
	"container/list"
	"net/http"
	"sync"
)

// responseCacheBytes bounds the memory published API responses take up.
const responseCacheBytes = 64 << 20

type cachedResponse struct {
	header http.Header
	body   []byte
}

// responseCache keeps published API responses by request URL. URLs carry the
// cv, so an entry never goes stale: content published later has a new cv and
// misses. Entries of older cvs leave as the least recently used.
type responseCache struct {
	mu       sync.Mutex
	maxBytes int
	bytes    int
	order    *list.List
	entries  map[string]*list.Element
}

type cacheEntry struct {
	key      string
	response cachedResponse
}

func newResponseCache(maxBytes int) *responseCache {
	return &responseCache{maxBytes: maxBytes, order: list.New(), entries: map[string]*list.Element{}}
}

func (e *cacheEntry) size() int { return len(e.key) + len(e.response.body) }

func (c *responseCache) get(key string) (cachedResponse, bool) {
	if c == nil {
		return cachedResponse{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.entries[key]
	if !ok {
		return cachedResponse{}, false
	}
	c.order.MoveToFront(element)
	return element.Value.(*cacheEntry).response, true
}

func (c *responseCache) add(key string, response cachedResponse) {
	if c == nil {
		return
	}
	entry := &cacheEntry{key: key, response: response}
	if entry.size() > c.maxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.remove(element)
	}
	c.entries[key] = c.order.PushFront(entry)
	c.bytes += entry.size()
	for c.bytes > c.maxBytes {
		c.remove(c.order.Back())
	}
}

func (c *responseCache) remove(element *list.Element) {
	entry := c.order.Remove(element).(*cacheEntry)
	delete(c.entries, entry.key)
	c.bytes -= entry.size()
}
