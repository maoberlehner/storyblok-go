// Package lru provides an in-memory cache that evicts the least recently used
// values once their total size exceeds a budget.
package lru

import (
	"container/list"
	"context"
	"fmt"
	"sync"
)

// Cache is safe for concurrent use. A nil *Cache caches nothing.
type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	maxSize int
	sizeOf  func(K, V) int
	size    int
	order   *list.List
	entries map[K]*list.Element
	loads   map[K]*load[V]
}

type entry[K comparable, V any] struct {
	key   K
	value V
	size  int
}

type load[V any] struct {
	done  chan struct{}
	value V
	err   error
}

// New returns a cache whose values' sizes, as sizeOf measures them, add up to
// at most maxSize.
func New[K comparable, V any](maxSize int, sizeOf func(K, V) int) *Cache[K, V] {
	return &Cache[K, V]{
		maxSize: maxSize,
		sizeOf:  sizeOf,
		order:   list.New(),
		entries: map[K]*list.Element{},
		loads:   map[K]*load[V]{},
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	if c == nil {
		var zero V
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.get(key)
}

func (c *Cache[K, V]) get(key K) (V, bool) {
	element, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(element)
	return element.Value.(*entry[K, V]).value, true
}

// Add caches value, unless it alone exceeds the budget.
func (c *Cache[K, V]) Add(key K, value V) {
	if c == nil {
		return
	}
	size := c.sizeOf(key, value)
	if size > c.maxSize {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.remove(element)
	}
	c.entries[key] = c.order.PushFront(&entry[K, V]{key: key, value: value, size: size})
	c.size += size
	for c.size > c.maxSize {
		c.remove(c.order.Back())
	}
}

func (c *Cache[K, V]) remove(element *list.Element) {
	e := c.order.Remove(element).(*entry[K, V])
	delete(c.entries, e.key)
	c.size -= e.size
}

// Load returns the cached value for key, or calls fn to load and cache it.
// Concurrent calls for the same key share one call of fn, which keeps running
// if the caller that started it gives up. Errors are not cached.
func (c *Cache[K, V]) Load(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, error) {
	if c == nil {
		return fn(ctx)
	}
	c.mu.Lock()
	if value, ok := c.get(key); ok {
		c.mu.Unlock()
		return value, nil
	}
	l, loading := c.loads[key]
	if !loading {
		l = &load[V]{done: make(chan struct{})}
		c.loads[key] = l
	}
	c.mu.Unlock()

	if !loading {
		go c.run(context.WithoutCancel(ctx), key, l, fn)
	}
	select {
	case <-l.done:
		return l.value, l.err
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

func (c *Cache[K, V]) run(ctx context.Context, key K, l *load[V], fn func(context.Context) (V, error)) {
	defer func() {
		// The load runs outside the request, where net/http can't recover a
		// panic; it fails the waiting requests instead of the process.
		if r := recover(); r != nil {
			l.err = fmt.Errorf("lru: load panicked: %v", r)
		}
		c.mu.Lock()
		delete(c.loads, key)
		c.mu.Unlock()
		close(l.done)
	}()
	l.value, l.err = fn(ctx)
	if l.err == nil {
		c.Add(key, l.value)
	}
}
