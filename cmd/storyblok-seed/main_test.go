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
	assets  map[string]map[string]any
	uploads int
}

func (f *fakeSpace) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.Contains(r.URL.Path, "/assets") || r.URL.Path == "/upload" {
		f.serveAssets(w, r)
		return
	}
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

// serveAssets mimics the signed upload flow: create, upload to storage, finish.
func (f *fakeSpace) serveAssets(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/assets"):
		var found []any
		if asset, ok := f.assets[r.URL.Query().Get("search")]; ok {
			found = append(found, asset)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"assets": found})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/assets"):
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		name := body["filename"].(string)
		f.nextID++
		pretty := "//a.storyblok.com/f/1/" + body["size"].(string) + "/abc/" + name
		f.assets[name] = map[string]any{"id": float64(f.nextID), "filename": "https:" + pretty}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": f.nextID, "pretty_url": pretty, "fields": map[string]string{"key": name},
			"post_url": "http://" + r.Host + "/upload",
		})
	case r.URL.Path == "/upload":
		f.uploads++
		w.WriteHeader(http.StatusNoContent)
	default:
		_, _ = w.Write([]byte(`{}`))
	}
}

// seedImagesUsed lists the seed images the story files reference.
func seedImagesUsed(t *testing.T) map[string]bool {
	t.Helper()
	files, err := readStories("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	data, _ := json.Marshal(files)
	for _, part := range strings.Split(string(data), `"seed_image":"`)[1:] {
		used[part[:strings.Index(part, `"`)]] = true
	}
	return used
}

func TestSeedingTwiceUpdatesTheSameStories(t *testing.T) {
	space := &fakeSpace{stories: map[string]map[string]any{}, assets: map[string]map[string]any{}}
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

	if space.uploads != len(seedImagesUsed(t)) {
		t.Errorf("uploaded %d images for %d used seed images; re-runs must reuse them", space.uploads, len(seedImagesUsed(t)))
	}
	hero, _ := json.Marshal(space.stories["landing/launch"]["content"])
	if !strings.Contains(string(hero), "/2400x1600/abc/seed-launch-hero.jpg") {
		t.Error("landing page does not reference the uploaded hero image with its size")
	}
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
