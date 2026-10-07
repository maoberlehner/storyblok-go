package server_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/server"
)

func TestPagesShowSiteChrome(t *testing.T) {
	_, body := do(t, http.MethodGet, newServer(t).URL+"/landing", "")
	for _, want := range []string{
		`<a class="site-header__skip" href="#main">`,
		`<a class="site-header__home" href="/">Acme</a>`,
		`popovertarget="site-nav"`,
		`<nav class="site-header__nav" id="site-nav" popover aria-label="Main">`,
		`href="/landing" aria-current="page">Landing</a>`,
		`href="https://docs.example.com">Docs</a>`,
		`<a class="base-button" href="/landing#contact">Contact</a>`,
		`<h2 class="site-link-group__heading">Company</h2>`,
		`href="/about">About</a>`,
		`href="https://example.com/privacy">Privacy</a>`,
		`<p class="site-footer__copyright">© 2026 Acme</p>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(body, `aria-current="page">`) != 1 {
		t.Error("want exactly one current navigation link")
	}
}

func TestSettingsStoryIsNotAPage(t *testing.T) {
	ts := newServer(t)
	if status, _ := do(t, http.MethodGet, ts.URL+"/settings", ""); status != http.StatusNotFound {
		t.Errorf("published settings: status %d, want 404", status)
	}
	status, body := do(t, http.MethodGet, ts.URL+"/settings"+previewQuery(), "")
	if status != http.StatusOK || !strings.Contains(body, `class="site-header__home"`) {
		t.Errorf("settings preview: status %d", status)
	}
}

func TestPagesRenderWithoutSettings(t *testing.T) {
	renderer, err := components.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	content := &fakeContent{stories: map[string]string{"home": homeStory}}
	srv := server.New(content, renderer, fstest.MapFS{}, &recordingInbox{}, previewToken, slog.New(slog.DiscardHandler), server.WithSiteURL(siteURL))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	status, body := do(t, http.MethodGet, ts.URL+"/", "")
	if status != http.StatusOK || strings.Contains(body, "site-header") || !strings.Contains(body, "<title>Welcome</title>") {
		t.Errorf("status %d, body:\n%s", status, body)
	}
}

func TestManifestUsesTheSiteName(t *testing.T) {
	_, body := do(t, http.MethodGet, newServer(t).URL+"/manifest.webmanifest", "")
	if !strings.Contains(body, `"name":"Acme"`) {
		t.Errorf("manifest: %s", body)
	}
}
