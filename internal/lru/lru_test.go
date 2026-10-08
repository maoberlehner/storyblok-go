package lru

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
)

func byteSize(key string, value []byte) int { return len(key) + len(value) }

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := New(12, byteSize)
	cache.Add("a", []byte("1234"))
	cache.Add("b", []byte("1234"))
	cache.Get("a")
	cache.Add("c", []byte("1234"))
	if _, ok := cache.Get("b"); ok {
		t.Error("b is cached, want it evicted")
	}
	for _, key := range []string{"a", "c"} {
		if _, ok := cache.Get(key); !ok {
			t.Errorf("%s is not cached", key)
		}
	}
}

func TestCacheSkipsValuesLargerThanItsBudget(t *testing.T) {
	cache := New(12, byteSize)
	cache.Add("a", []byte("1234"))
	cache.Add("big", []byte("1234567890"))
	if _, ok := cache.Get("big"); ok {
		t.Error("caches a value larger than the budget")
	}
	if _, ok := cache.Get("a"); !ok {
		t.Error("an oversized value evicted a")
	}
}

func TestCacheReplacesValues(t *testing.T) {
	cache := New(12, byteSize)
	cache.Add("a", []byte("1234"))
	cache.Add("a", []byte("5678"))
	cache.Add("b", []byte("1234"))
	if value, _ := cache.Get("a"); string(value) != "5678" {
		t.Errorf("a = %q, want 5678", value)
	}
	if _, ok := cache.Get("b"); !ok {
		t.Error("the replaced value still counts against the budget")
	}
}

func TestNilCacheCachesNothing(t *testing.T) {
	var cache *Cache[string, []byte]
	cache.Add("a", []byte("1"))
	if _, ok := cache.Get("a"); ok {
		t.Error("nil cache returned a value")
	}
	loads := 0
	for range 2 {
		if _, err := cache.Load(t.Context(), "a", func(context.Context) ([]byte, error) {
			loads++
			return []byte("1"), nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if loads != 2 {
		t.Errorf("%d loads, want 2", loads)
	}
}

func TestLoadCachesTheLoadedValue(t *testing.T) {
	cache := New(100, byteSize)
	loads := 0
	for range 2 {
		value, err := cache.Load(t.Context(), "a", func(context.Context) ([]byte, error) {
			loads++
			return []byte("1"), nil
		})
		if err != nil || string(value) != "1" {
			t.Fatalf("Load = %q, %v", value, err)
		}
	}
	if loads != 1 {
		t.Errorf("%d loads, want 1", loads)
	}
}

func TestLoadSharesConcurrentLoadsOfOneKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := New(100, byteSize)
		release := make(chan struct{})
		loads := 0
		load := func(context.Context) ([]byte, error) {
			loads++
			<-release
			return []byte("1"), nil
		}
		var wg sync.WaitGroup
		for range 3 {
			wg.Go(func() {
				if value, err := cache.Load(t.Context(), "a", load); err != nil || string(value) != "1" {
					t.Errorf("Load = %q, %v", value, err)
				}
			})
		}
		synctest.Wait()
		close(release)
		wg.Wait()
		if loads != 1 {
			t.Errorf("%d loads, want 1", loads)
		}
	})
}

func TestLoadDoesNotCacheErrors(t *testing.T) {
	cache := New(100, byteSize)
	failure := errors.New("unavailable")
	if _, err := cache.Load(t.Context(), "a", func(context.Context) ([]byte, error) { return nil, failure }); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}
	value, err := cache.Load(t.Context(), "a", func(context.Context) ([]byte, error) { return []byte("1"), nil })
	if err != nil || string(value) != "1" {
		t.Errorf("Load after a failure = %q, %v", value, err)
	}
}

func TestLoadOutlivesTheCallerThatStartedIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := New(100, byteSize)
		release := make(chan struct{})
		load := func(ctx context.Context) ([]byte, error) {
			<-release
			return []byte("1"), ctx.Err()
		}
		first, cancel := context.WithCancel(t.Context())
		firstErr := make(chan error)
		go func() {
			_, err := cache.Load(first, "a", load)
			firstErr <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-firstErr; !errors.Is(err, context.Canceled) {
			t.Errorf("canceled caller: err = %v, want %v", err, context.Canceled)
		}

		second := make(chan []byte)
		go func() {
			value, _ := cache.Load(t.Context(), "a", load)
			second <- value
		}()
		synctest.Wait()
		close(release)
		if value := <-second; string(value) != "1" {
			t.Errorf("waiting caller got %q, want 1", value)
		}
	})
}
