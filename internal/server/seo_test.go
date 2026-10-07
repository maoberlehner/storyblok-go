package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestCanonicalURL(t *testing.T) {
	ts := newServer(t)
	for path, want := range map[string]string{
		"/":                                 siteURL + "/",
		"/home":                             siteURL + "/",
		"/landing?page-arts=2&sent=contact": siteURL + "/landing",
	} {
		body := request(t, http.MethodGet, ts.URL+path, nil, nil).body
		if tag := `<link rel="canonical" href="` + want + `">`; !strings.Contains(body, tag) {
			t.Errorf("%s: missing %s", path, tag)
		}
		if strings.Contains(body, "noindex") {
			t.Errorf("%s: published page is noindex", path)
		}
	}
}

func TestPreviewsAreNotIndexed(t *testing.T) {
	ts := newServer(t)
	body := request(t, http.MethodGet, ts.URL+"/"+previewQuery(), nil, nil).body
	if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Error("preview lacks noindex")
	}
	if strings.Contains(body, `rel="canonical"`) {
		t.Error("preview has a canonical URL, which conflicts with noindex")
	}
}

func TestSitemap(t *testing.T) {
	ts := newServer(t)
	res := request(t, http.MethodGet, ts.URL+"/sitemap.xml", nil, nil)
	if res.status != http.StatusOK || !strings.HasPrefix(res.header.Get("Content-Type"), "application/xml") {
		t.Fatalf("status %d, Content-Type %q", res.status, res.header.Get("Content-Type"))
	}
	for _, want := range []string{
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">`,
		"<url><loc>https://example.com/</loc><lastmod>2026-10-01T08:30:00Z</lastmod>",
		"<url><loc>https://example.com/landing</loc>",
	} {
		if !strings.Contains(res.body, want) {
			t.Errorf("sitemap does not contain %s:\n%s", want, res.body)
		}
	}
	if strings.Contains(res.body, "legacy") {
		t.Error("sitemap lists a story without a page component")
	}
	if res.header.Get("CDN-Cache-Control") == "" {
		t.Error("sitemap is not cacheable")
	}
}

func TestRobotsPointToSitemap(t *testing.T) {
	ts := newServer(t)
	res := request(t, http.MethodGet, ts.URL+"/robots.txt", nil, nil)
	if res.status != http.StatusOK || !strings.Contains(res.body, "Sitemap: https://example.com/sitemap.xml") {
		t.Errorf("status %d, body:\n%s", res.status, res.body)
	}
}
