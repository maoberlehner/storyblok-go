package storyblok

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestClientLearnsAndUpdatesCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		responses := []int64{100, 200, 150, 0}
		queries := []string{"", "100", "200", "200"}
		calls := 0
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("cv"); got != queries[calls] {
				t.Errorf("request %d cv = %q, want %q", calls, got, queries[calls])
			}
			body := fmt.Sprintf(`{"story":{"name":"Home"},"cv":%d}`, responses[calls])
			calls++
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(body))), nil
		})
		for range responses {
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestClientRefreshesCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		queries := []string{"", "100", "", "200"}
		calls := 0
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("cv"); got != queries[calls] {
				t.Errorf("request %d cv = %q, want %q", calls, got, queries[calls])
			}
			cv := 100
			if calls >= 2 {
				cv = 200
			}
			calls++
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(fmt.Sprintf(`{"story":{},"cv":%d}`, cv)))), nil
		})
		for i := range queries {
			if i == 2 {
				time.Sleep(30 * time.Second)
			}
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestClientKeepsDraftAndSpaceRequestsSeparate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		calls := 0
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			want := ""
			if calls == 3 {
				want = "100"
			}
			if got := r.URL.Query().Get("cv"); got != want {
				t.Errorf("request %d cv = %q, want %q", calls, got, want)
			}
			cv := 100
			if r.URL.Query().Get("version") == "draft" {
				cv = 900
			}
			calls++
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(fmt.Sprintf(`{"story":{},"cv":%d,"space":{"id":123}}`, cv)))), nil
		})
		for _, version := range []Version{Published, Draft} {
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: version}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.SpaceID(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestClientAdaptsRateToCacheHitsAndThrottling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		var times []time.Time
		throttle := false
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			times = append(times, time.Now())
			if throttle {
				throttle = false
				return apiResponse(429, http.Header{"Retry-After": {"0"}}, io.NopCloser(strings.NewReader("error"))), nil
			}
			return apiResponse(200, http.Header{"X-Cache": {"Hit from cloudfront"}}, io.NopCloser(strings.NewReader(`{"story":{},"cv":100}`))), nil
		})
		get := func() {
			t.Helper()
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
		get()
		get()
		if gap := times[1].Sub(times[0]); gap < 40*time.Millisecond {
			t.Fatalf("initial rate too fast: %v", gap)
		}
		// Sustained hits over several seconds should permit more than 25/s.
		for range 20 {
			get()
		}
		time.Sleep(time.Second)
		get()
		get()
		if gap := times[len(times)-1].Sub(times[len(times)-2]); gap >= 40*time.Millisecond {
			t.Errorf("cache hits did not raise the rate: gap = %v", gap)
		}
		throttle = true
		get()
		get()
		if gap := times[len(times)-1].Sub(times[len(times)-2]); gap < 40*time.Millisecond {
			t.Errorf("429 did not reduce the rate: gap = %v", gap)
		}
	})
}

func TestRateReductionSlowsAlreadyQueuedRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := newAdaptiveRateLimiter()
		limiter.limiter.SetLimit(1000)
		request, err := http.NewRequestWithContext(t.Context(), "GET", DefaultBaseURL+"/stories/home?version=published&cv=100", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := limiter.wait(request); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		var wg sync.WaitGroup
		var mu sync.Mutex
		var times []time.Time
		for range 3 {
			wg.Go(func() {
				if err := limiter.wait(request); err != nil {
					t.Error(err)
				}
				mu.Lock()
				times = append(times, time.Now())
				mu.Unlock()
			})
		}
		synctest.Wait()
		limiter.observe(request, apiResponse(429, nil, http.NoBody))
		wg.Wait()
		previous := start
		for _, at := range times {
			if gap := at.Sub(previous); gap < 40*time.Millisecond {
				t.Errorf("queued request used the old rate: gap = %v", gap)
			}
			previous = at
		}
	})
}

func TestCachedRateHasCeilingAndCooldown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := newAdaptiveRateLimiter()
		request, err := http.NewRequest("GET", DefaultBaseURL+"/stories/home?version=published&cv=100", nil)
		if err != nil {
			t.Fatal(err)
		}
		hit := apiResponse(200, http.Header{"X-Cache": {"Hit from cloudfront"}}, http.NoBody)
		for range 10 {
			time.Sleep(time.Second)
			for range 10 {
				limiter.observe(request, hit)
			}
		}
		if got := limiter.limiter.Limit(); got != 1000 {
			t.Fatalf("cached rate = %v, want 1000", got)
		}
		limiter.observe(request, apiResponse(429, nil, http.NoBody))
		for range 20 {
			limiter.observe(request, hit)
		}
		if got := limiter.limiter.Limit(); got != 25 {
			t.Errorf("in-flight hits reversed the 429 reduction: rate = %v", got)
		}
		time.Sleep(5 * time.Second)
		limiter.observe(request, hit)
		if got := limiter.limiter.Limit(); got <= 25 {
			t.Errorf("rate did not recover after cooldown: %v", got)
		}
		for range 10 {
			limiter.observe(request, apiResponse(429, nil, http.NoBody))
		}
		if got := limiter.limiter.Limit(); got != 1 {
			t.Errorf("rate = %v after repeated 429s, want floor of 1", got)
		}
	})
}

func TestMissesAndUnknownHeadersLeaveCachedTier(t *testing.T) {
	for _, cache := range []string{"", "Miss from cloudfront", "Error from cloudfront"} {
		t.Run(cache, func(t *testing.T) {
			limiter := newAdaptiveRateLimiter()
			limiter.limiter.SetLimit(1000)
			request, err := http.NewRequest("GET", DefaultBaseURL+"/stories/home?version=published&cv=100", nil)
			if err != nil {
				t.Fatal(err)
			}
			limiter.observe(request, apiResponse(200, http.Header{"X-Cache": {cache}}, http.NoBody))
			if got := limiter.limiter.Limit(); got != 25 {
				t.Errorf("rate = %v, want 25", got)
			}
		})
	}
}

func TestUncachedRequestsCannotUseCachedAllowance(t *testing.T) {
	for _, path := range []string{"/spaces/me", "/stories/home?version=draft&cv=100", "/stories/home?version=published"} {
		t.Run(path, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				limiter := newAdaptiveRateLimiter()
				limiter.limiter.SetLimit(1000)
				request, err := http.NewRequestWithContext(t.Context(), "GET", DefaultBaseURL+path, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := limiter.wait(request); err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				if err := limiter.wait(request); err != nil {
					t.Fatal(err)
				}
				if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
					t.Errorf("uncached request used CDN allowance: elapsed = %v", elapsed)
				}
			})
		})
	}
}

func TestConcurrentResponsesCannotRegressCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		started := make(chan struct{})
		release := make(chan struct{})
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			cv := 100
			switch {
			case strings.HasSuffix(r.URL.Path, "/slow"):
				close(started)
				<-release
				cv = 200
			case strings.HasSuffix(r.URL.Path, "/fast"):
				cv = 300
			case strings.HasSuffix(r.URL.Path, "/last"):
				if got := r.URL.Query().Get("cv"); got != "300" {
					t.Errorf("last request cv = %q, want 300", got)
				}
				cv = 300
			}
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(fmt.Sprintf(`{"story":{},"cv":%d}`, cv)))), nil
		})
		get := func(slug string) {
			t.Helper()
			if _, err := client.Story(t.Context(), slug, StoryOptions{Version: Published}); err != nil {
				t.Error(err)
			}
		}
		get("home")
		var wg sync.WaitGroup
		wg.Go(func() { get("slow") })
		<-started
		get("fast")
		close(release)
		wg.Wait()
		get("last")
	})
}
