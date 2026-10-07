package mapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListPagination(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Total", "2")
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = w.Write([]byte(`{"components":[{"id":9007199254740993,"name":"page-one"}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"components":[{"id":9007199254740994,"name":"page-two"}]}`))
		default:
			t.Error("unexpected page")
		}
	}))
	defer ts.Close()
	c, err := NewClient(ts.URL, "123", "token")
	if err != nil {
		t.Fatal(err)
	}
	items, err := c.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != 9007199254740993 || calls != 2 {
		t.Fatalf("pagination/IDs: %#v; calls=%d", items, calls)
	}
}

func TestRateLimitRetryAndCancellation(t *testing.T) {
	t.Run("retry", func(t *testing.T) {
		calls := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(429)
				return
			}
			_, _ = w.Write([]byte(`{"components":[]}`))
		}))
		defer ts.Close()
		c, _ := NewClient(ts.URL, "123", "token")
		if _, err := c.List(t.Context()); err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("calls=%d", calls)
		}
	})
	t.Run("cancel backoff", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(429) }))
		defer ts.Close()
		c, _ := NewClient(ts.URL, "123", "token")
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		if _, err := c.List(ctx); err == nil {
			t.Fatal("expected cancellation")
		}
		if time.Since(start) > time.Second {
			t.Fatal("backoff ignored context")
		}
	})
}

func TestNoRetryOfAmbiguousCreateAndNoCredentialLeak(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(500)
		_, _ = w.Write([]byte("secret-token"))
	}))
	defer ts.Close()
	c, _ := NewClient(ts.URL, "123", "secret-token")
	_, err := c.Write(t.Context(), 0, desiredComponents()[0])
	if err == nil || calls != 1 || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRedirectDoesNotForwardToken(t *testing.T) {
	received := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	c, _ := NewClient(source.URL, "123", "token")
	if _, err := c.List(t.Context()); err == nil {
		t.Fatal("redirect accepted")
	}
	if received {
		t.Fatal("followed credentialed redirect")
	}
}
