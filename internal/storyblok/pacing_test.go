package storyblok

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

var (
	baseInterval = time.Duration(float64(time.Second) / float64(baseRequestRate))
	baseBurst    = baseRequestRate
)

// fakeCDN answers published story requests with the X-Cache header in
// cacheHeader and everything else with a plain 200. A request to a slug in
// status gets that status instead.
type fakeCDN struct {
	mu          sync.Mutex
	cacheHeader string
	status      map[string]int
	sent        []time.Time
}

func (f *fakeCDN) set(cacheHeader string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cacheHeader = cacheHeader
}

func (f *fakeCDN) roundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, time.Now())
	slug := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if status, ok := f.status[slug]; ok {
		return apiResponse(status, http.Header{"Retry-After": {"0"}}, io.NopCloser(strings.NewReader("error"))), nil
	}
	return storyResponse(100, http.Header{"X-Cache": {f.cacheHeader}}), nil
}

func newFakeCDNClient(cacheHeader string) (*Client, *fakeCDN) {
	cdn := &fakeCDN{cacheHeader: cacheHeader, status: map[string]int{}}
	client := NewClient(DefaultBaseURL, "secret")
	client.api.HTTPClient.Transport = roundTripFunc(cdn.roundTrip)
	// Pacing applies to the requests that reach the API.
	client.responses = nil
	return client, cdn
}

// requestsWithin sends sequential requests for d and returns how many
// succeeded.
func requestsWithin(t *testing.T, d time.Duration, get func() error) int {
	t.Helper()
	count := 0
	for start := time.Now(); time.Since(start) < d; {
		if err := get(); err == nil {
			count++
		}
	}
	return count
}

func published(t *testing.T, client *Client, slug string) func() error {
	return func() error {
		_, err := client.Story(t.Context(), slug, StoryOptions{Version: Published})
		return err
	}
}

func TestClientStartsAtBaseRate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, _ := newFakeCDNClient("Miss from cloudfront")
		got := requestsWithin(t, 2*time.Second, published(t, client, "home"))
		// One extra request discovers the cv.
		if want := 1 + baseBurst + 2*int(baseRequestRate); got > want {
			t.Errorf("%d requests in 2s, want at most %d", got, want)
		}
	})
}

func TestClientGrowsRateWithCacheHitsUpToCeiling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, _ := newFakeCDNClient(cloudFrontCacheHit)
		requestsWithin(t, 10*time.Second, published(t, client, "home"))
		got := requestsWithin(t, time.Second, published(t, client, "home"))
		if got < int(maxCachedRate)/2 || got > int(maxCachedRate)+baseBurst {
			t.Errorf("%d requests per second after sustained hits, want about %v", got, maxCachedRate)
		}
	})
}

func TestClientReturnsToBaseRateWithoutCacheHits(t *testing.T) {
	for name, header := range map[string]string{"miss": "Miss from cloudfront", "error": "Error from cloudfront", "missing": ""} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				client, cdn := newFakeCDNClient(cloudFrontCacheHit)
				requestsWithin(t, 10*time.Second, published(t, client, "home"))
				cdn.set(header)
				got := requestsWithin(t, time.Second, published(t, client, "home"))
				if limit := 2 * int(baseRequestRate); got > limit {
					t.Errorf("%d requests per second after misses, want at most %d", got, limit)
				}
			})
		})
	}
}

func TestClientIgnoresNotFoundResponsesForPacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, cdn := newFakeCDNClient(cloudFrontCacheHit)
		cdn.status["missing"] = http.StatusNotFound
		requests := 0
		requestsWithin(t, 10*time.Second, func() error {
			requests++
			if requests%50 == 0 {
				return published(t, client, "missing")()
			}
			return published(t, client, "home")()
		})
		if got := requestsWithin(t, time.Second, published(t, client, "home")); got < int(maxCachedRate)/2 {
			t.Errorf("%d requests per second with occasional 404s, want about %v", got, maxCachedRate)
		}
	})
}

func TestDraftsDoNotSlowPublishedRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, cdn := newFakeCDNClient(cloudFrontCacheHit)
		requestsWithin(t, 10*time.Second, published(t, client, "home"))
		cdn.status["draft"] = http.StatusTooManyRequests
		var wg sync.WaitGroup
		for range 50 {
			wg.Go(func() { _, _ = client.Story(t.Context(), "draft", StoryOptions{Version: Draft}) })
		}
		got := requestsWithin(t, time.Second, published(t, client, "home"))
		wg.Wait()
		if got < int(maxCachedRate)/2 {
			t.Errorf("%d published requests per second during throttled drafts, want about %v", got, maxCachedRate)
		}
	})
}

func TestClientHandlesColdStartBurst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, _ := newFakeCDNClient(cloudFrontCacheHit)
		var failed atomic.Int32
		var wg sync.WaitGroup
		for range 2 * baseBurst {
			wg.Go(func() {
				if err := published(t, client, "home")(); err != nil {
					failed.Add(1)
				}
			})
		}
		wg.Wait()
		if got := failed.Load(); got > 0 {
			t.Errorf("%d of %d concurrent requests failed on a cold client", got, 2*baseBurst)
		}
	})
}

func TestClientFailsFastWhenRateLimitWaitIsTooLong(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, _ := newFakeCDNClient("")
		errs := make(chan error, 200)
		var wg sync.WaitGroup
		for range cap(errs) {
			wg.Go(func() {
				_, err := client.Story(t.Context(), "home", StoryOptions{Version: Draft})
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		limited := 0
		for err := range errs {
			if err == nil {
				continue
			}
			if !errors.Is(err, ErrRateLimited) {
				t.Fatalf("err = %v, want ErrRateLimited", err)
			}
			if want := `storyblok: fetching story "home": rate-limit wait exceeded`; err.Error() != want {
				t.Fatalf("err = %q, want %q", err, want)
			}
			limited++
		}
		if limited == 0 {
			t.Error("no request failed, want the excess to fail with ErrRateLimited")
		}
	})
}
