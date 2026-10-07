package storyblok

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const apiPayload = `{"story":{"name":"Home"},"space":{"id":123}}`

func apiResponse(status int, header http.Header, body io.ReadCloser) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: header, Body: body}
}

func TestClientRetriesTransientResponses(t *testing.T) {
	for _, status := range []int{429, 500, 502, 503, 504} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := 0
				client := NewClient(DefaultBaseURL, "secret")
				client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					attempts++
					code := http.StatusOK
					if attempts == 1 {
						code = status
					}
					return apiResponse(code, nil, io.NopCloser(strings.NewReader(apiPayload))), nil
				})
				if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
					t.Fatal(err)
				}
				if attempts != 2 {
					t.Fatalf("attempts = %d, want 2", attempts)
				}
			})
		})
	}
}

func TestClientDoesNotRetryPermanentResponses(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 501} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			attempts := 0
			client := NewClient(DefaultBaseURL, "secret")
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				attempts++
				return apiResponse(status, nil, io.NopCloser(strings.NewReader("error"))), nil
			})
			_, err := client.Story(t.Context(), "home", StoryOptions{Version: Published})
			if err == nil || attempts != 1 {
				t.Fatalf("err = %v, attempts = %d, want error and one attempt", err, attempts)
			}
			if status == http.StatusNotFound && !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestClientLimitsRetriesAndClosesBodies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var bodies []*trackedBody
		client := NewClient(DefaultBaseURL, "secret")
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if len(bodies) > 0 && !bodies[len(bodies)-1].closed {
				t.Fatal("previous response body was not closed before retrying")
			}
			body := &trackedBody{Reader: strings.NewReader("error")}
			bodies = append(bodies, body)
			return apiResponse(http.StatusServiceUnavailable, nil, body), nil
		})
		_, err := client.Story(t.Context(), "home", StoryOptions{Version: Published})
		if err == nil || !strings.Contains(err.Error(), "Service Unavailable") {
			t.Fatalf("err = %v, want final HTTP status", err)
		}
		if len(bodies) != 4 {
			t.Fatalf("attempts = %d, want 4", len(bodies))
		}
		for i, body := range bodies {
			if !body.closed {
				t.Errorf("response body %d was not closed", i)
			}
		}
	})
}

func TestClientRespectsRetryAfter(t *testing.T) {
	for _, date := range []bool{false, true} {
		name := "seconds"
		if date {
			name = "HTTP date"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				attempts := 0
				client := NewClient(DefaultBaseURL, "secret")
				client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					attempts++
					if attempts == 1 {
						after := "2"
						if date {
							after = start.Add(2 * time.Second).UTC().Format(http.TimeFormat)
						}
						return apiResponse(429, http.Header{"Retry-After": {after}}, io.NopCloser(strings.NewReader("error"))), nil
					}
					if time.Since(start) < 2*time.Second {
						t.Errorf("retried before Retry-After: %v", time.Since(start))
					}
					return apiResponse(200, nil, io.NopCloser(strings.NewReader(apiPayload))), nil
				})
				if id, err := client.SpaceID(t.Context()); err != nil || id != 123 {
					t.Fatalf("SpaceID = %d, %v", id, err)
				}
				if attempts != 2 {
					t.Fatalf("attempts = %d, want 2", attempts)
				}
			})
		})
	}
}

func TestClientSharesRateLimitAcrossRequestsAndRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var times []time.Time
		client := NewClient(DefaultBaseURL, "secret")
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			times = append(times, time.Now())
			if len(times) == 1 {
				return apiResponse(429, http.Header{"Retry-After": {"0"}}, io.NopCloser(strings.NewReader("error"))), nil
			}
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(apiPayload))), nil
		})
		var wg sync.WaitGroup
		for range 3 {
			wg.Go(func() {
				if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
					t.Error(err)
				}
			})
			wg.Go(func() {
				if _, err := client.SpaceID(t.Context()); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		if len(times) != 7 {
			t.Fatalf("attempts = %d, want 7", len(times))
		}
		for i := 1; i < len(times); i++ {
			if gap := times[i].Sub(times[i-1]); gap < 40*time.Millisecond {
				t.Errorf("attempts %d and %d spaced %v apart, want at least 40ms", i-1, i, gap)
			}
		}
	})
}

func TestClientDeadlineIncludesRetryWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		body := &trackedBody{Reader: strings.NewReader("error")}
		client := NewClient(DefaultBaseURL, "secret")
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			return apiResponse(429, http.Header{"Retry-After": {"60"}}, body), nil
		})
		start := time.Now()
		_, err := client.Story(t.Context(), "home", StoryOptions{Version: Published})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want deadline exceeded", err)
		}
		if elapsed := time.Since(start); elapsed != 10*time.Second {
			t.Errorf("elapsed = %v, want 10s", elapsed)
		}
		if attempts != 1 || !body.closed {
			t.Errorf("attempts = %d, body.closed = %v, want 1 and true", attempts, body.closed)
		}
	})
}

func TestClientRetriesTransportFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		client := NewClient(DefaultBaseURL, "secret-token")
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 4 {
				return nil, io.ErrUnexpectedEOF
			}
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(apiPayload))), nil
		})
		if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
			t.Fatal(err)
		}
		if attempts != 4 {
			t.Errorf("attempts = %d, want 4", attempts)
		}
	})
}

func TestClientCancellationStopsWaiting(t *testing.T) {
	for _, retry := range []bool{false, true} {
		name := "rate limiter"
		if retry {
			name = "retry backoff"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := 0
				client := NewClient(DefaultBaseURL, "secret-token")
				client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					attempts++
					if retry {
						return apiResponse(429, http.Header{"Retry-After": {"60"}}, io.NopCloser(strings.NewReader("error"))), nil
					}
					return apiResponse(200, nil, io.NopCloser(strings.NewReader(apiPayload))), nil
				})
				if !retry {
					// Consume the immediate permit so the next call has to wait.
					if _, err := client.SpaceID(t.Context()); err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				result := make(chan error, 1)
				go func() {
					_, err := client.Story(ctx, "home", StoryOptions{Version: Published})
					result <- err
				}()
				synctest.Wait()
				cancel()
				if err := <-result; !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "secret-token") {
					t.Fatalf("err = %v, want cancellation without token", err)
				}
				if attempts != 1 {
					t.Errorf("attempts = %d, want 1", attempts)
				}
			})
		})
	}
}

func TestClientDoesNotRetryCertificateErrors(t *testing.T) {
	for _, certErr := range []error{
		x509.UnknownAuthorityError{},
		x509.CertificateInvalidError{Cert: &x509.Certificate{}, Reason: x509.Expired},
		x509.HostnameError{Certificate: &x509.Certificate{}, Host: "api.storyblok.com"},
	} {
		t.Run(certErr.Error(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := 0
				client := NewClient(DefaultBaseURL, "secret-token")
				client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					attempts++
					return nil, certErr
				})
				_, err := client.Story(t.Context(), "home", StoryOptions{Version: Published})
				if err == nil || attempts != 1 || strings.Contains(err.Error(), "secret-token") {
					t.Errorf("err = %v, attempts = %d, want certificate error without token and one attempt", err, attempts)
				}
			})
		})
	}
}
