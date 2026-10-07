// Package apihttp implements the HTTP behavior shared by the Storyblok API
// clients: request pacing, retries, and timeouts.
package apihttp

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/failsafehttp"
)

const (
	maxRetries     = 3
	baseRetryDelay = 200 * time.Millisecond
	maxRetryDelay  = 2 * time.Second
	maxDrainBytes  = 64 << 10
	maxRedirects   = 10
)

// Pacer grants permits for requests and adapts to their responses.
type Pacer interface {
	Wait(*http.Request) error
	// Observe receives the response to the final request after redirects.
	Observe(*http.Response)
}

type Config struct {
	Pacer Pacer
	// AttemptTimeout bounds each attempt, including reading the response
	// body, so a stalled attempt leaves time for retries.
	AttemptTimeout time.Duration
	// FollowRedirects follows redirects and paces each of them. Otherwise
	// redirect responses are returned as they are.
	FollowRedirects bool
}

// Client sends requests with pacing and retries. Reads are retried on
// transport errors, 429, and 5xx responses other than 501. Writes are only
// retried on 429, where the server did not process them; other failures leave
// their outcome unknown.
type Client struct {
	// HTTPClient sends each attempt. Tests replace its Transport.
	HTTPClient *http.Client
	pacer      Pacer
	executor   failsafe.Executor[*http.Response]
}

func NewClient(config Config) *Client {
	httpClient := &http.Client{
		Timeout: config.AttemptTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if !config.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return config.Pacer.Wait(request)
		},
	}
	return &Client{HTTPClient: httpClient, pacer: config.Pacer, executor: newExecutor()}
}

// errRetryAfterExceedsDeadline stops retrying when the server asks for a
// longer wait than the request has left, so the caller gets the status
// immediately.
var errRetryAfterExceedsDeadline = errors.New("Retry-After exceeds the deadline")

// notRetryableError marks the outcome of a write that the server may have
// processed.
type notRetryableError struct{ err error }

func (e notRetryableError) Error() string {
	if e.err == nil {
		return "not retryable"
	}
	return e.err.Error()
}

func (e notRetryableError) Unwrap() error { return e.err }

func newExecutor() failsafe.Executor[*http.Response] {
	retry := failsafehttp.NewRetryPolicyBuilder().
		WithMaxRetries(maxRetries).
		WithDelayFunc(retryDelay).
		AbortOnErrors(ErrRateLimited, errRetryAfterExceedsDeadline).
		AbortOnErrorTypes(notRetryableError{}, x509.UnknownAuthorityError{}, x509.CertificateInvalidError{}, x509.HostnameError{}, (*tls.CertificateVerificationError)(nil)).
		ReturnLastFailure().
		OnRetryScheduled(func(event failsafe.ExecutionScheduledEvent[*http.Response]) {
			if response := event.LastResult(); response != nil {
				// Release failed responses before waiting, including when the wait
				// is canceled. Drain so the connection can be reused.
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxDrainBytes))
				_ = response.Body.Close()
			}
		}).Build()

	return failsafe.With(retry)
}

// Do sends request. Errors never contain the request URL, which can carry an
// access token.
func (c *Client) Do(request *http.Request) (*http.Response, error) {
	response, err := c.executor.WithContext(request.Context()).Get(func() (*http.Response, error) {
		attempt := request
		if request.GetBody != nil {
			body, err := request.GetBody()
			if err != nil {
				return nil, notRetryableError{err}
			}
			attempt = request.Clone(request.Context())
			attempt.Body = body
		}
		// Acquire on every attempt so retries use the same budget.
		if err := c.pacer.Wait(attempt); err != nil {
			return nil, err
		}
		response, err := c.HTTPClient.Do(attempt)
		if response != nil {
			if response.Request == nil {
				response.Request = attempt
			}
			c.pacer.Observe(response)
		}
		if delay, ok := retryAfter(response); ok {
			if deadline, ok := request.Context().Deadline(); ok && time.Now().Add(delay).After(deadline) {
				return response, errRetryAfterExceedsDeadline
			}
		}
		if !isRead(request.Method) && (err != nil || response.StatusCode != http.StatusTooManyRequests) {
			return response, notRetryableError{err}
		}
		return response, err
	})
	if errors.Is(err, errRetryAfterExceedsDeadline) {
		return response, nil
	}
	if notRetryable, ok := errors.AsType[notRetryableError](err); ok {
		err = notRetryable.err
	}
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		err = urlErr.Err
	}
	return response, err
}

func isRead(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func retryDelay(attempt failsafe.ExecutionAttempt[*http.Response]) time.Duration {
	if delay, ok := retryAfter(attempt.LastResult()); ok {
		return delay
	}
	delay := min(baseRetryDelay<<attempt.Retries(), maxRetryDelay)
	maxJitter := delay / 4
	return delay + rand.N(maxJitter+1)
}

// retryAfter returns the delay a 429 or 503 response asks for. It is used
// as-is, without jitter.
func retryAfter(response *http.Response) (time.Duration, bool) {
	if response == nil ||
		(response.StatusCode != http.StatusTooManyRequests && response.StatusCode != http.StatusServiceUnavailable) {
		return 0, false
	}
	after := response.Header.Get("Retry-After")
	if seconds, err := strconv.ParseUint(after, 10, 32); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	if date, err := http.ParseTime(after); err == nil {
		return max(0, time.Until(date)), true
	}
	return 0, false
}
