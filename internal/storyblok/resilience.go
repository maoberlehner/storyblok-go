package storyblok

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/failsafehttp"
)

const (
	operationTimeout = 10 * time.Second
	// attemptTimeout leaves room for retries when a single attempt stalls.
	attemptTimeout = 4 * time.Second
	maxRetries     = 3
	baseRetryDelay = 200 * time.Millisecond
	maxRetryDelay  = 2 * time.Second
	maxDrainBytes  = 64 << 10
	maxRedirects   = 10
)

func newHTTPClient(pacing *adaptiveRateLimiter) *http.Client {
	return &http.Client{
		Timeout: attemptTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			// Storyblok redirects published requests without a current cv.
			// The redirected request counts against the rate limit, too.
			return pacing.wait(request)
		},
	}
}

func newHTTPExecutor() failsafe.Executor[*http.Response] {
	retry := failsafehttp.NewRetryPolicyBuilder().
		WithMaxRetries(maxRetries).
		WithDelayFunc(retryDelay).
		AbortOnErrors(ErrRateLimited, errRetryAfterExceedsDeadline).
		AbortOnErrorTypes(x509.UnknownAuthorityError{}, x509.CertificateInvalidError{}, x509.HostnameError{}, (*tls.CertificateVerificationError)(nil)).
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

// errRetryAfterExceedsDeadline stops retrying when the server asks for a
// longer wait than the operation has left, so the caller gets the status
// immediately.
var errRetryAfterExceedsDeadline = errors.New("Retry-After exceeds the deadline")

func (c *Client) do(request *http.Request) (*http.Response, error) {
	response, err := c.executor.WithContext(request.Context()).Get(func() (*http.Response, error) {
		// Acquire on every attempt so retries use the same adaptive budget.
		if err := c.pacing.wait(request); err != nil {
			return nil, err
		}
		response, err := c.httpClient.Do(request)
		c.pacing.observe(request, response)
		if delay, ok := retryAfter(response); ok {
			if deadline, ok := request.Context().Deadline(); ok && time.Now().Add(delay).After(deadline) {
				return response, errRetryAfterExceedsDeadline
			}
		}
		return response, err
	})
	if errors.Is(err, errRetryAfterExceedsDeadline) {
		return response, nil
	}
	return response, err
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
