package server_test

import (
	"context"
	"encoding/json/jsontext"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/server"
	"storyblok-go-website/internal/storyblok"
)

// seedContent serves the seed stories as the response cache would: raw JSON
// without network or parsing costs.
type seedContent struct {
	stories map[string][]byte
	cv      int64
}

func (c seedContent) Stories(context.Context, storyblok.StoriesOptions) (storyblok.StoryList, error) {
	return storyblok.StoryList{Stories: []byte(`[{"full_slug":"articles/a","content":{"title":"A"}}]`), Total: 1}, nil
}

func (c seedContent) CacheVersion() (int64, bool) { return c.cv, c.cv > 0 }

func (c seedContent) Story(_ context.Context, slug string, _ storyblok.StoryOptions) (jsontext.Value, error) {
	if story, ok := c.stories[slug]; ok {
		return story, nil
	}
	return nil, storyblok.ErrNotFound
}

func BenchmarkShowStory(b *testing.B) {
	stories := map[string][]byte{}
	for _, slug := range []string{"home", "settings"} {
		raw, err := os.ReadFile("../../seed/" + slug + ".json")
		if err != nil {
			b.Fatal(err)
		}
		stories[slug] = raw
	}
	// Without a cv, every request decodes the story.
	for _, bc := range []struct {
		name string
		cv   int64
	}{{"decoded per request", 0}, {"decoded once", 1}} {
		b.Run(bc.name, func(b *testing.B) {
			renderer, err := components.NewRenderer()
			if err != nil {
				b.Fatal(err)
			}
			handler := server.New(seedContent{stories, bc.cv}, renderer, fstest.MapFS{}, nil, "", slog.New(slog.DiscardHandler)).Handler()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
					if rec.Code != http.StatusOK {
						b.Errorf("status = %d", rec.Code)
					}
				}
			})
		})
	}
}
