package server

import (
	"net/http"
	"strings"
)

// ContentSecurityPolicy blocks plugins, <base> hijacking, foreign form
// targets, plain HTTP, and framing by anyone but the Visual Editor. Scripts,
// styles, and connections stay open to any HTTPS origin, inline code, and
// eval, because tag managers and the snippets marketing adds through them
// depend on all three (ADR 0004).
const ContentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' 'unsafe-eval' https:; " +
	"style-src 'self' 'unsafe-inline' https:; " +
	"img-src 'self' data: blob: https:; " +
	"font-src 'self' data: https:; " +
	"connect-src 'self' https:; " +
	"frame-src 'self' https:; " +
	"media-src 'self' https:; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'self' https://app.storyblok.com; " +
	"upgrade-insecure-requests"

// securityHeaders sets the headers every response carries. HSTS is only sent
// for HTTPS origins; it has no includeSubDomains, so other subdomains keep
// their own policy.
func securityHeaders(next http.Handler, siteURL string) http.Handler {
	hsts := strings.HasPrefix(siteURL, "https://")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", ContentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
