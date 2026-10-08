package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseSiteURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.example.com":  "https://www.example.com",
		"https://www.example.com/": "https://www.example.com",
		"http://localhost:8080":    "http://localhost:8080",
	} {
		if got, err := parseSiteURL(raw); err != nil || got != want {
			t.Errorf("parseSiteURL(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "www.example.com", "ftp://example.com", "https://example.com/blog", "https://example.com/?a=1"} {
		if _, err := parseSiteURL(raw); err == nil {
			t.Errorf("parseSiteURL(%q) accepted an invalid origin", raw)
		}
	}
}

// logBuffer collects the log output of a server running in another
// goroutine.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// setRunEnv configures run with free local addresses and a Storyblok API
// that is never reached.
func setRunEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	api := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(api.Close)
	env := map[string]string{
		"STORYBLOK_PREVIEW_TOKEN": "token",
		"FORM_SECRET":             "secret",
		"SITE_URL":                "https://www.example.com",
		"STORYBLOK_API_URL":       api.URL,
		"ADDR":                    "127.0.0.1:0",
		"METRICS_ADDR":            "127.0.0.1:0",
		"DEV_TOOLBAR":             "",
		"TLS_CERT_FILE":           "",
		"TLS_KEY_FILE":            "",
	}
	for k, v := range overrides {
		env[k] = v
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestParseLogLevel(t *testing.T) {
	for raw, want := range map[string]slog.Level{"": slog.LevelInfo, "warn": slog.LevelWarn, "DEBUG": slog.LevelDebug, "error": slog.LevelError} {
		if got, err := parseLogLevel(raw); err != nil || got != want {
			t.Errorf("parseLogLevel(%q) = %v, %v; want %v", raw, got, err, want)
		}
	}
	if _, err := parseLogLevel("loud"); err == nil {
		t.Error("accepted LOG_LEVEL=loud")
	}
}

func TestRunReportsATakenAddressWithoutListening(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	setRunEnv(t, map[string]string{"ADDR": taken.Addr().String()})
	var logs logBuffer
	if err := run(t.Context(), slog.New(slog.NewTextHandler(&logs, nil))); err == nil {
		t.Fatal("run succeeded on a taken address")
	}
	if strings.Contains(logs.String(), "listening") {
		t.Errorf("logged listening:\n%s", logs.String())
	}
}

var listeningAddr = regexp.MustCompile(`msg=listening addr=(\S+)`)

func TestRunServesUntilTheContextEnds(t *testing.T) {
	setRunEnv(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	var logs logBuffer
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.New(slog.NewTextHandler(&logs, nil))) }()

	var addr string
	for deadline := time.Now().Add(10 * time.Second); addr == ""; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("not listening:\n%s", logs.String())
		}
		if m := listeningAddr.FindStringSubmatch(logs.String()); m != nil {
			addr = m[1]
		}
	}
	res, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if strings.TrimSpace(string(body)) != "ok" {
		t.Errorf("GET /healthz = %d %q", res.StatusCode, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run = %v", err)
		}
	case <-time.After(shutdownTimeout):
		t.Fatal("run did not return")
	}
	if !strings.Contains(logs.String(), "shutting down") {
		t.Errorf("no shutdown log:\n%s", logs.String())
	}
}
