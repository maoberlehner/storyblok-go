package server_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/server"
	"storyblok-go-website/internal/storyblok"
)

const (
	siteURL      = "https://example.com"
	previewToken = "preview-token"
	rateLimited  = "rate-limited"
	landingStory = `{"id": 8, "name": "Landing", "content": {
		"component": "page-landing-page", "_uid": "l1", "title": "Landing",
		"sections": [
			{"component": "block-section-articles", "_uid": "arts", "heading": "Latest", "folder": "articles"},
			{"component": "block-section-contact", "_uid": "contact", "heading": "Contact us"},
			{"component": "block-section-articles", "_uid": "more", "heading": "More", "folder": "articles"}
		]
	}}`
	settingsStory = `{"id": 3, "name": "Settings", "content": {
		"component": "site-settings", "_uid": "s1", "site_name": "Acme",
		"navigation": [
			{"component": "site-link", "_uid": "n1", "label": "Landing", "link": {"linktype": "story", "cached_url": "landing"}},
			{"component": "site-link", "_uid": "n2", "label": "Docs", "link": {"linktype": "url", "url": "https://docs.example.com"}}
		],
		"cta_label": "Contact", "cta_link": {"linktype": "story", "cached_url": "landing", "anchor": "contact"},
		"footer_columns": [{"component": "site-link-group", "_uid": "g1", "heading": "Company",
			"links": [{"component": "site-link", "_uid": "n3", "label": "About", "link": {"linktype": "story", "cached_url": "about"}}]}],
		"legal_links": [{"component": "site-link", "_uid": "n4", "label": "Privacy", "link": {"linktype": "url", "url": "https://example.com/privacy"}}],
		"copyright": "© 2026 Acme"
	}}`
	homeStory = `{"id": 7, "name": "Home", "content": {
		"component": "page-landing-page", "_uid": "u1",
		"title": "Welcome",
		"_editable": "<!--#storyblok#{\"name\": \"page-landing-page\", \"uid\": \"u1\", \"id\": \"1\"}-->",
		"sections": [{"component": "not_built_yet", "_uid": "u2", "_editable": "<!--#storyblok#{\"name\": \"not_built_yet\", \"uid\": \"u2\", \"id\": \"1\"}-->"}]
	}}`
)

// fakeContent mirrors the Content Delivery API, which only includes editable
// markers in draft content.
type fakeContent struct {
	stories   map[string]string
	cv        atomic.Int64
	confirmed atomic.Bool
	fetches   atomic.Int64
}

const totalArticles = 14

// Stories returns totalArticles articles, newest ("article-14") first, or all
// stories when listing the whole space, as for the sitemap.
func (f *fakeContent) Stories(_ context.Context, opts storyblok.StoriesOptions) (storyblok.StoryList, error) {
	var stories []map[string]any
	if opts.StartsWith == "" {
		for slug, raw := range f.stories {
			var story map[string]any
			if json.Unmarshal([]byte(raw), &story) != nil {
				continue
			}
			story["full_slug"] = slug
			story["published_at"] = "2026-10-01T08:30:00.000Z"
			stories = append(stories, story)
		}
		raw, err := json.Marshal(stories)
		return storyblok.StoryList{Stories: raw, Total: len(stories)}, err
	}
	first := (opts.Page-1)*opts.PerPage + 1
	for n := first; n < first+opts.PerPage && n <= totalArticles; n++ {
		id := totalArticles + 1 - n
		stories = append(stories, map[string]any{
			"full_slug": fmt.Sprintf("%sarticle-%d", opts.StartsWith, id),
			"content":   map[string]any{"component": "page-article", "title": fmt.Sprintf("Article %d", id), "description": "Teaser"},
		})
	}
	raw, err := json.Marshal(stories)
	return storyblok.StoryList{Stories: raw, Total: totalArticles}, err
}

func (f *fakeContent) CacheVersion() (int64, bool) { return f.cv.Load(), f.confirmed.Load() }

func (f *fakeContent) Story(_ context.Context, slug string, opts storyblok.StoryOptions) (jsontext.Value, error) {
	f.fetches.Add(1)
	story, ok := f.stories[slug]
	if !ok {
		return nil, storyblok.ErrNotFound
	}
	if story == rateLimited {
		return nil, fmt.Errorf("storyblok: fetching story %q: %w", slug, storyblok.ErrRateLimited)
	}
	var tree map[string]any
	if err := json.Unmarshal([]byte(story), &tree); err != nil {
		return nil, err
	}
	tree["full_slug"] = slug
	if opts.Version != storyblok.Draft {
		removeEditable(tree)
	}
	return json.Marshal(tree)
}

func removeEditable(node any) {
	switch n := node.(type) {
	case map[string]any:
		delete(n, "_editable")
		for _, child := range n {
			removeEditable(child)
		}
	case []any:
		for _, child := range n {
			removeEditable(child)
		}
	}
}

type recordingInbox struct{ submissions []components.Submission }

func (i *recordingInbox) Deliver(_ context.Context, s components.Submission) error {
	i.submissions = append(i.submissions, s)
	return nil
}

func newServer(t *testing.T, opts ...components.RendererOption) *httptest.Server {
	t.Helper()
	ts, _ := newServerWithContent(t, &recordingInbox{}, nil, opts...)
	return ts
}

func newServerWithInbox(t *testing.T, inbox components.Inbox, opts ...components.RendererOption) *httptest.Server {
	t.Helper()
	ts, _ := newServerWithContent(t, inbox, nil, opts...)
	return ts
}

func newServerWithContent(t *testing.T, inbox components.Inbox, serverOpts []server.Option, opts ...components.RendererOption) (*httptest.Server, *fakeContent) {
	t.Helper()
	renderer, err := components.NewRenderer(opts...)
	if err != nil {
		t.Fatal(err)
	}
	content := &fakeContent{stories: map[string]string{
		"home": homeStory, "settings": settingsStory, "busy": rateLimited, "landing": landingStory,
		"legacy": `{"name": "Legacy", "content": {"component": "page", "body": []}}`,
	}}
	serverOpts = append([]server.Option{server.WithSiteURL(siteURL)}, serverOpts...)
	srv := server.New(content, renderer, fstest.MapFS{}, inbox, previewToken, slog.New(slog.DiscardHandler), serverOpts...)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, content
}

func previewQuery() string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha1.Sum([]byte("1:" + previewToken + ":" + ts))
	return "?" + url.Values{
		"_storyblok_tk[space_id]":  {"1"},
		"_storyblok_tk[timestamp]": {ts},
		"_storyblok_tk[token]":     {hex.EncodeToString(sum[:])},
	}.Encode()
}

func do(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestShowStory(t *testing.T) {
	ts := newServer(t)

	t.Run("renders the story at the root path as a full document", func(t *testing.T) {
		status, body := do(t, http.MethodGet, ts.URL+"/", "")
		if status != http.StatusOK || !strings.Contains(body, "<title>Welcome</title>") {
			t.Errorf("status %d, body:\n%s", status, body)
		}
		if strings.Contains(body, "data-blok-c") || strings.Contains(body, "storyblok-v2-latest.js") {
			t.Error("published page contains Visual Editor markup")
		}
	})

	t.Run("adds Visual Editor markup in preview", func(t *testing.T) {
		_, body := do(t, http.MethodGet, ts.URL+"/"+previewQuery(), "")
		for _, want := range []string{`data-blok-uid="1-u1"`, "storyblok-v2-latest.js", "No component for <code>not_built_yet</code>"} {
			if !strings.Contains(body, want) {
				t.Errorf("body does not contain %q", want)
			}
		}
	})

	t.Run("responds 404 for unknown slugs", func(t *testing.T) {
		if status, _ := do(t, http.MethodGet, ts.URL+"/missing", ""); status != http.StatusNotFound {
			t.Errorf("status = %d, want 404", status)
		}
	})

	t.Run("responds 503 when the Storyblok client is rate limited", func(t *testing.T) {
		if status, _ := do(t, http.MethodGet, ts.URL+"/busy", ""); status != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", status)
		}
	})
}

func get(t *testing.T, url string, header http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(req.Header, header)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func TestPageCaching(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, []server.Option{server.WithBuildID("b1")})
	content.cv.Store(100)
	content.confirmed.Store(true)

	t.Run("lets browsers revalidate and the shared cache serve stale pages", func(t *testing.T) {
		res := get(t, ts.URL+"/", nil)
		if got := res.Header.Get("ETag"); got != `"b1-100"` {
			t.Errorf("ETag = %q", got)
		}
		if got := res.Header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q", got)
		}
		if got := res.Header.Get("CDN-Cache-Control"); !strings.Contains(got, "stale-while-revalidate=") || !strings.Contains(got, "stale-if-error=") {
			t.Errorf("CDN-Cache-Control = %q", got)
		}
	})

	t.Run("answers a current ETag with 304 without fetching content", func(t *testing.T) {
		before := content.fetches.Load()
		for _, etag := range []string{`"b1-100"`, `W/"b1-100"`, `"other", W/"b1-100"`} {
			res := get(t, ts.URL+"/", http.Header{"If-None-Match": {etag}})
			if res.StatusCode != http.StatusNotModified || res.Header.Get("ETag") != `"b1-100"` {
				t.Errorf("If-None-Match %s: status %d, ETag %q", etag, res.StatusCode, res.Header.Get("ETag"))
			}
		}
		if fetched := content.fetches.Load() - before; fetched != 0 {
			t.Errorf("fetched content %d times", fetched)
		}
	})

	t.Run("renders the page for an ETag of an older content version", func(t *testing.T) {
		content.cv.Store(101)
		t.Cleanup(func() { content.cv.Store(100) })
		res := get(t, ts.URL+"/", http.Header{"If-None-Match": {`"b1-100"`}})
		if res.StatusCode != http.StatusOK || res.Header.Get("ETag") != `"b1-101"` {
			t.Errorf("status %d, ETag %q", res.StatusCode, res.Header.Get("ETag"))
		}
	})

	t.Run("renders the page while the content version is unconfirmed", func(t *testing.T) {
		content.confirmed.Store(false)
		t.Cleanup(func() { content.confirmed.Store(true) })
		if res := get(t, ts.URL+"/", http.Header{"If-None-Match": {`"b1-100"`}}); res.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", res.StatusCode)
		}
	})

	t.Run("never caches previews", func(t *testing.T) {
		res := get(t, ts.URL+"/"+previewQuery(), http.Header{"If-None-Match": {`"b1-100"`}})
		if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("ETag") != "" || res.Header.Get("CDN-Cache-Control") != "" {
			t.Errorf("status %d, headers %v", res.StatusCode, res.Header)
		}
	})

	t.Run("does not mark missing pages cacheable", func(t *testing.T) {
		res := get(t, ts.URL+"/missing", nil)
		if res.StatusCode != http.StatusNotFound || res.Header.Get("CDN-Cache-Control") != "" {
			t.Errorf("status %d, headers %v", res.StatusCode, res.Header)
		}
	})
}

func TestPreviewStory(t *testing.T) {
	ts := newServer(t)
	edited := strings.Replace(homeStory, `"sections": [`, `"sections": [{"component": "edited_block", "_editable": "<!--#storyblok#{}-->"},`, 1)

	t.Run("renders the posted story as the main content fragment", func(t *testing.T) {
		status, body := do(t, http.MethodPut, ts.URL+"/"+previewQuery(), edited)
		if status != http.StatusOK {
			t.Fatalf("status = %d, body: %s", status, body)
		}
		if !strings.Contains(body, "<code>edited_block</code>") || strings.Contains(body, "<html") {
			t.Errorf("body is not the rendered content fragment:\n%s", body)
		}
	})

	t.Run("rejects requests without a valid preview token", func(t *testing.T) {
		if status, _ := do(t, http.MethodPut, ts.URL+"/", edited); status != http.StatusForbidden {
			t.Errorf("status = %d, want 403", status)
		}
	})
}

func TestDevToolbar(t *testing.T) {
	ts := newServer(t, components.WithDevToolbar(99))

	t.Run("marks blocks for opening them in the Visual Editor", func(t *testing.T) {
		_, body := do(t, http.MethodGet, ts.URL+"/", "")
		for _, want := range []string{`data-dev-blok="u1"`, `data-space-id="99"`, `data-story-id="7"`, "dev-toolbar.js"} {
			if !strings.Contains(body, want) {
				t.Errorf("body does not contain %q", want)
			}
		}
	})

	t.Run("is left out inside the Visual Editor", func(t *testing.T) {
		_, body := do(t, http.MethodGet, ts.URL+"/"+previewQuery(), "")
		if strings.Contains(body, "dev-toolbar.js") || strings.Contains(body, "data-dev-blok") {
			t.Error("preview page contains dev toolbar markup")
		}
	})
}
