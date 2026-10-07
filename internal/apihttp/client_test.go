package apihttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"golang.org/x/time/rate"
)

const testRate = 20

var testInterval = time.Second / testRate

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, header http.Header) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: header, Body: io.NopCloser(strings.NewReader("{}"))}
}

var throttled = http.Header{"Retry-After": {"0"}}

func newTestClient(rt roundTripFunc) *Client {
	tier := NewTier(TierConfig{Base: testRate, MaxBurst: testRate, MaxWait: 2 * time.Second})
	client := NewClient(Config{Pacer: tier, AttemptTimeout: time.Second})
	client.HTTPClient.Transport = rt
	return client
}

func get(t *testing.T, client *Client, path string) (*http.Response, error) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if response != nil {
		response.Body.Close()
	}
	return response, err
}

// requestsWithin sends sequential requests for d and returns how many
// succeeded.
func requestsWithin(t *testing.T, client *Client, d time.Duration) int {
	t.Helper()
	count := 0
	for start := time.Now(); time.Since(start) < d; {
		if response, err := get(t, client, "/"); err == nil && response.StatusCode == http.StatusOK {
			count++
		}
	}
	return count
}

func TestTierRecoversFromThrottling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		status := http.StatusTooManyRequests
		client := newTestClient(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			return respond(status, throttled), nil
		})
		requestsWithin(t, client, 6*time.Second)
		mu.Lock()
		status = http.StatusOK
		mu.Unlock()
		if got := requestsWithin(t, client, time.Second); got > 2 {
			t.Errorf("%d requests per second after sustained 429s, want the floor of %v", got, minRequestRate)
		}
		requestsWithin(t, client, throttleCooldown+6*growthInterval)
		if got := requestsWithin(t, client, time.Second); got < testRate-1 {
			t.Errorf("%d requests per second after recovering, want about %d", got, testRate)
		}
	})
}

func TestTierHalvesRateOncePerBurstOfThrottledResponses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const concurrent = testRate / 2
		var mu sync.Mutex
		seen := map[string]bool{}
		allSent := make(chan struct{})
		client := newTestClient(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			if seen[r.URL.Path] {
				mu.Unlock()
				return respond(http.StatusOK, nil), nil
			}
			seen[r.URL.Path] = true
			if len(seen) == concurrent {
				close(allSent)
			}
			mu.Unlock()
			// Every first attempt is on the wire before the first 429 arrives.
			<-allSent
			return respond(http.StatusTooManyRequests, throttled), nil
		})
		var wg sync.WaitGroup
		for i := range concurrent {
			wg.Go(func() {
				if response, err := get(t, client, fmt.Sprintf("/%d", i)); err != nil || response.StatusCode != http.StatusOK {
					t.Errorf("request %d: %v", i, err)
				}
			})
		}
		wg.Wait()
		if got := requestsWithin(t, client, time.Second); got < testRate/2-1 {
			t.Errorf("%d requests per second after one burst of 429s, want about %d", got, testRate/2)
		}
	})
}

func TestTierSlowsQueuedRequestsAfterThrottling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var sent []time.Time
		throttleAt := testRate + 5
		client := newTestClient(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			sent = append(sent, time.Now())
			if len(sent) == throttleAt {
				return respond(http.StatusTooManyRequests, throttled), nil
			}
			return respond(http.StatusOK, nil), nil
		})
		var wg sync.WaitGroup
		for range throttleAt + 10 {
			wg.Go(func() { _, _ = get(t, client, "/") })
		}
		wg.Wait()
		for i := throttleAt; i < len(sent); i++ {
			if gap := sent[i].Sub(sent[i-1]); gap < 2*testInterval {
				t.Errorf("request %d sent %v after the previous one, want at least %v", i, gap, 2*testInterval)
			}
		}
	})
}

func TestTierFailsWhenPermitWaitIsTooLong(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tier := NewTier(TierConfig{Base: rate.Limit(1), MaxBurst: 1, MaxWait: time.Second})
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		errs := make(chan error, 3)
		for range cap(errs) {
			go func() { errs <- tier.Wait(request) }()
		}
		var limited int
		for range cap(errs) {
			if err := <-errs; errors.Is(err, ErrRateLimited) {
				limited++
			}
		}
		if limited != 1 {
			t.Errorf("%d of 3 waits failed at 1/s with a one-second wait, want 1", limited)
		}
	})
}

func TestClientRetriesReads(t *testing.T) {
	for _, failure := range []func() (*http.Response, error){
		func() (*http.Response, error) { return respond(http.StatusBadGateway, nil), nil },
		func() (*http.Response, error) { return nil, io.ErrUnexpectedEOF },
		func() (*http.Response, error) { return respond(http.StatusTooManyRequests, throttled), nil },
	} {
		synctest.Test(t, func(t *testing.T) {
			attempts := 0
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					return failure()
				}
				return respond(http.StatusOK, nil), nil
			})
			if response, err := get(t, client, "/"); err != nil || response.StatusCode != http.StatusOK || attempts != 2 {
				t.Errorf("response = %v, err = %v, attempts = %d, want 200 after one retry", response, err, attempts)
			}
		})
	}
}

func TestClientRetriesWritesOnlyWhenThrottled(t *testing.T) {
	post := func(t *testing.T, client *Client) (*http.Response, error) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://api.example.com", strings.NewReader("payload"))
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if response != nil {
			response.Body.Close()
		}
		return response, err
	}

	t.Run("throttled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var bodies []string
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(r.Body)
				bodies = append(bodies, string(body))
				if len(bodies) == 1 {
					return respond(http.StatusTooManyRequests, throttled), nil
				}
				return respond(http.StatusCreated, nil), nil
			})
			response, err := post(t, client)
			if err != nil || response.StatusCode != http.StatusCreated {
				t.Fatalf("response = %v, err = %v, want 201", response, err)
			}
			if fmt.Sprint(bodies) != "[payload payload]" {
				t.Errorf("bodies = %q, want the payload resent", bodies)
			}
		})
	})

	for name, failure := range map[string]func() (*http.Response, error){
		"server error":    func() (*http.Response, error) { return respond(http.StatusInternalServerError, nil), nil },
		"transport error": func() (*http.Response, error) { return nil, io.ErrUnexpectedEOF },
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := 0
				client := newTestClient(func(r *http.Request) (*http.Response, error) {
					attempts++
					return failure()
				})
				response, err := post(t, client)
				if attempts != 1 {
					t.Errorf("attempts = %d, want 1", attempts)
				}
				if (response == nil || response.StatusCode != http.StatusInternalServerError) && err == nil {
					t.Errorf("response = %v, err = %v, want the failure", response, err)
				}
			})
		})
	}
}

func TestClientReturnsThrottlingStatusWhenRetryAfterExceedsDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		client := newTestClient(func(r *http.Request) (*http.Response, error) {
			attempts++
			return respond(http.StatusTooManyRequests, http.Header{"Retry-After": {"60"}}), nil
		})
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		response, err := client.Do(request)
		if response != nil {
			response.Body.Close()
		}
		if err != nil || response.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("response = %v, err = %v, want the 429", response, err)
		}
		if elapsed := time.Since(start); elapsed != 0 || attempts != 1 {
			t.Errorf("elapsed = %v, attempts = %d, want an immediate response", elapsed, attempts)
		}
	})
}

func TestClientReturnsRedirectsWhenNotFollowing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var paths []string
		client := newTestClient(func(r *http.Request) (*http.Response, error) {
			paths = append(paths, r.URL.Path)
			return respond(http.StatusFound, http.Header{"Location": {"https://elsewhere.example.com/target"}}), nil
		})
		response, err := get(t, client, "/source")
		if err != nil || response.StatusCode != http.StatusFound || fmt.Sprint(paths) != "[/source]" {
			t.Errorf("response = %v, err = %v, paths = %q, want the redirect unfollowed", response, err, paths)
		}
	})
}

func TestClientErrorsOmitRequestURL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := newTestClient(func(r *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})
		_, err := get(t, client, "/?token=secret-token")
		if err == nil || strings.Contains(err.Error(), "secret-token") {
			t.Errorf("err = %v, want an error without the URL", err)
		}
	})
}
