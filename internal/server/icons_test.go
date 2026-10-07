package server_test

import (
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/server"
)

func newServerWithIcons(t *testing.T) *httptest.Server {
	t.Helper()
	renderer, err := components.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	assets := fstest.MapFS{
		"icons/favicon.ico":          {Data: []byte("ico")},
		"icons/apple-touch-icon.png": {Data: []byte("png")},
	}
	content := &fakeContent{stories: map[string]string{"home": homeStory}}
	srv := server.New(content, renderer, assets, &recordingInbox{}, previewToken, slog.New(slog.DiscardHandler), server.WithSiteURL(siteURL))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestIconsAtWellKnownPaths(t *testing.T) {
	ts := newServerWithIcons(t)
	for path, body := range map[string]string{"/favicon.ico": "ico", "/apple-touch-icon.png": "png"} {
		res := request(t, http.MethodGet, ts.URL+path, nil, nil)
		if res.status != http.StatusOK || res.body != body || !strings.Contains(res.header.Get("Cache-Control"), "max-age=86400") {
			t.Errorf("%s: status %d, body %q, headers %v", path, res.status, res.body, res.header)
		}
	}
}

func TestManifest(t *testing.T) {
	ts := newServerWithIcons(t)
	res := request(t, http.MethodGet, ts.URL+"/manifest.webmanifest", nil, nil)
	if res.status != http.StatusOK || res.header.Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("status %d, Content-Type %q", res.status, res.header.Get("Content-Type"))
	}
	var manifest struct {
		Name  string `json:"name"`
		Start string `json:"start_url"`
		Icons []struct {
			Sizes   string `json:"sizes"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal([]byte(res.body), &manifest); err != nil {
		t.Fatal(err)
	}
	var sizes []string
	for _, icon := range manifest.Icons {
		sizes = append(sizes, icon.Sizes+":"+icon.Purpose)
	}
	if manifest.Name == "" || manifest.Start != "/" || strings.Join(sizes, ",") != "192x192:any,512x512:any,512x512:maskable" {
		t.Errorf("manifest = %+v", manifest)
	}
}

func TestPagesLinkIcons(t *testing.T) {
	_, body := do(t, http.MethodGet, newServer(t).URL+"/", "")
	for _, want := range []string{
		`<link rel="icon" href="/favicon.ico" sizes="32x32">`,
		`<link rel="icon" href="/assets/icons/icon.svg" type="image/svg+xml">`,
		`<link rel="apple-touch-icon" href="/apple-touch-icon.png">`,
		`<link rel="manifest" href="/manifest.webmanifest">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}
