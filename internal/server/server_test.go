package server_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/server"
	"storyblok-go-website/internal/storyblok"
)

const (
	previewToken = "preview-token"
	homeStory    = `{"id": 7, "name": "Home", "content": {
		"component": "page-landing-page", "_uid": "u1",
		"title": "Welcome",
		"_editable": "<!--#storyblok#{\"name\": \"page-landing-page\", \"uid\": \"u1\", \"id\": \"1\"}-->",
		"sections": [{"component": "not_built_yet", "_uid": "u2", "_editable": "<!--#storyblok#{\"name\": \"not_built_yet\", \"uid\": \"u2\", \"id\": \"1\"}-->"}]
	}}`
)

// fakeContent mirrors the Content Delivery API, which only includes editable
// markers in draft content.
type fakeContent map[string]string

func (f fakeContent) Story(_ context.Context, slug string, opts storyblok.StoryOptions) (jsontext.Value, error) {
	story, ok := f[slug]
	if !ok {
		return nil, storyblok.ErrNotFound
	}
	if opts.Version == storyblok.Draft {
		return jsontext.Value(story), nil
	}
	var tree any
	if err := json.Unmarshal([]byte(story), &tree); err != nil {
		return nil, err
	}
	removeEditable(tree)
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

func newServer(t *testing.T, opts ...components.RendererOption) *httptest.Server {
	t.Helper()
	renderer, err := components.NewRenderer(opts...)
	if err != nil {
		t.Fatal(err)
	}
	content := fakeContent{"home": homeStory}
	srv := server.New(content, renderer, fstest.MapFS{}, previewToken, slog.New(slog.DiscardHandler))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
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
