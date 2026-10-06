package storyblok

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/failsafehttp"
)

const requestTimeout = 10 * time.Second

func newHTTPExecutor() failsafe.Executor[*http.Response] {
	retry := failsafehttp.NewRetryPolicyBuilder().
		WithMaxRetries(3).
		WithDelayFunc(retryDelay).
		AbortOnErrors(context.DeadlineExceeded, errRateLimitWait).
		AbortOnErrorTypes(x509.UnknownAuthorityError{}, x509.CertificateInvalidError{}, x509.HostnameError{}, (*tls.CertificateVerificationError)(nil)).
		ReturnLastFailure().
		OnRetryScheduled(func(event failsafe.ExecutionScheduledEvent[*http.Response]) {
			if response := event.LastResult(); response != nil {
				// Release failed responses before waiting, including when the wait
				// is canceled. A bounded drain permits connection reuse without
				// reading an arbitrarily large error response.
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
				_ = response.Body.Close()
			}
		}).Build()

	return failsafe.With(retry)
}

func (c *Client) do(request *http.Request) (*http.Response, error) {
	return c.executor.WithContext(request.Context()).Get(func() (*http.Response, error) {
		// Acquire on every attempt so retries use the same adaptive budget.
		if err := c.pacing.wait(request); err != nil {
			return nil, err
		}
		response, err := c.httpClient.Do(request)
		c.pacing.observe(request, response)
		return response, err
	})
}

func retryDelay(attempt failsafe.ExecutionAttempt[*http.Response]) time.Duration {
	if response := attempt.LastResult(); response != nil &&
		(response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable) {
		after := response.Header.Get("Retry-After")
		if seconds, err := strconv.ParseUint(after, 10, 32); err == nil {
			return time.Duration(seconds) * time.Second
		}
		if date, err := http.ParseTime(after); err == nil {
			return max(0, time.Until(date))
		}
	}

	// Exponential backoff with up to 25% additive jitter. Keep jitter off
	// Retry-After delays so we never retry earlier than the server permits.
	delay := min(200*time.Millisecond*time.Duration(1<<attempt.Retries()), 2*time.Second)
	return delay + time.Duration(rand.Int64N(int64(delay/4)+1))
}
