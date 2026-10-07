package server_test

import (
	"net/http"
	"strings"
	"testing"

	"storyblok-go-website/internal/server"
)

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	ts := newServer(t)
	for _, path := range []string{"/", "/missing", "/robots.txt", "/assets/app.js", "/" + previewQuery()} {
		res := request(t, http.MethodGet, ts.URL+path, nil, nil)
		h := res.header
		if got := h.Get("Content-Security-Policy"); got != server.ContentSecurityPolicy {
			t.Errorf("%s: CSP = %q", path, got)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
			t.Errorf("%s: missing nosniff or referrer policy: %v", path, h)
		}
		if got := h.Get("Strict-Transport-Security"); got != "max-age=31536000" {
			t.Errorf("%s: HSTS = %q", path, got)
		}
	}
}

func TestContentSecurityPolicyKeepsMarketingToolsWorking(t *testing.T) {
	for _, want := range []string{
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' https:",
		"object-src 'none'", "base-uri 'self'", "form-action 'self'",
		"frame-ancestors 'self' https://app.storyblok.com", "upgrade-insecure-requests",
	} {
		if !strings.Contains(server.ContentSecurityPolicy, want) {
			t.Errorf("CSP lacks %q", want)
		}
	}
}

func TestNoHSTSForHTTPOrigins(t *testing.T) {
	// Options apply in order, so this replaces the default site URL.
	ts, _ := newServerWithContent(t, &recordingInbox{}, []server.Option{server.WithSiteURL("http://localhost:8080")})
	res := request(t, http.MethodGet, ts.URL+"/", nil, nil)
	if got := res.header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS on an HTTP origin: %q", got)
	}
}
