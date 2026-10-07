package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeSpace keeps stories by full slug, like the Management API.
type fakeSpace struct {
	mu      sync.Mutex
	stories map[string]map[string]any
	nextID  int64
	writes  []string
}

func (f *fakeSpace) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method == http.MethodGet {
		var found []any
		if story, ok := f.stories[r.URL.Query().Get("with_slug")]; ok {
			found = append(found, story)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"stories": found})
		return
	}
	var body struct {
		Story   map[string]any `json:"story"`
		Publish int            `json:"publish"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	story := body.Story
	parentSlug := ""
	for slug, s := range f.stories {
		if s["id"] == story["parent_id"] {
			parentSlug = slug
		}
	}
	fullSlug := strings.TrimPrefix(path.Join(parentSlug, story["slug"].(string)), "/")
	if r.Method == http.MethodPost {
		f.nextID++
		story["id"] = float64(f.nextID)
	} else {
		id, _ := strconv.ParseFloat(path.Base(r.URL.Path), 64)
		story["id"] = id
	}
	story["full_slug"] = fullSlug
	f.stories[fullSlug] = story
	f.writes = append(f.writes, r.Method+" "+fullSlug+" publish="+strconv.Itoa(body.Publish))
	_ = json.NewEncoder(w).Encode(map[string]any{"story": story})
}

func TestSeedingTwiceUpdatesTheSameStories(t *testing.T) {
	space := &fakeSpace{stories: map[string]map[string]any{}}
	api := httptest.NewServer(space)
	defer api.Close()
	env := func(key string) string {
		return map[string]string{"STORYBLOK_TOKEN": "token", "STORYBLOK_SPACE": "1"}[key]
	}
	seed := func() {
		t.Helper()
		var out bytes.Buffer
		if err := run(t.Context(), []string{"--api-url", api.URL, "--dir", "../../seed"}, env, &out); err != nil {
			t.Fatal(err)
		}
	}

	seed()
	created := len(space.stories)
	firstContent, _ := json.Marshal(space.stories["landing/launch"]["content"])
	space.writes = nil
	seed()

	if len(space.stories) != created {
		t.Errorf("second run changed the number of stories from %d to %d", created, len(space.stories))
	}
	for _, write := range space.writes {
		if !strings.HasPrefix(write, "PUT ") || !strings.HasSuffix(write, "publish=1") {
			t.Errorf("second run wrote %q, want only published updates", write)
		}
	}
	if folder := space.stories["articles"]; folder["is_folder"] != true {
		t.Errorf("articles folder = %v", folder)
	}
	if got := len(articles()); got < 2*6+1 {
		t.Errorf("%d articles do not fill more than two pages", got)
	}
	if _, ok := space.stories["articles/a-practical-guide-to-design-tokens"]; !ok {
		t.Error("generated article is missing")
	}
	secondContent, _ := json.Marshal(space.stories["landing/launch"]["content"])
	if !bytes.Equal(firstContent, secondContent) {
		t.Error("seeded content changed between runs")
	}
}
