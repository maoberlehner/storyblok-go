package components

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"testing"

	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

type benchArticles struct{}

func (benchArticles) Stories(_ context.Context, opts storyblok.StoriesOptions) (storyblok.StoryList, error) {
	var stories []map[string]any
	for i := range opts.PerPage {
		stories = append(stories, map[string]any{
			"full_slug": fmt.Sprintf("blog/article-%d", i),
			"content":   map[string]any{"title": fmt.Sprintf("Article %d", i), "description": "A short teaser."},
		})
	}
	raw, err := json.Marshal(stories)
	return storyblok.StoryList{Stories: raw, Total: 20}, err
}

func readSeed[T any](b testing.TB, name string) storyblok.Story[T] {
	b.Helper()
	data, err := os.ReadFile("../../seed/" + name + ".json")
	if err != nil {
		b.Fatal(err)
	}
	var story storyblok.Story[T]
	if err := json.Unmarshal(data, &story); err != nil {
		b.Fatal(err)
	}
	return story
}

// BenchmarkPage renders the seeded launch page, which uses nearly every
// component, with the site chrome.
func BenchmarkPage(b *testing.B) {
	story := readSeed[AnyBlock](b, "landing-launch")
	settings := readSeed[AnyBlock](b, "settings").Content.Block.(*SiteSettings)
	req := Request{Path: "/landing-launch", Locale: locale.Default, FormToken: "token"}
	loaded, err := LoadSections(context.Background(), benchArticles{}, story.Content.Block, req)
	if err != nil {
		b.Fatal(err)
	}
	r, err := NewRenderer()
	if err != nil {
		b.Fatal(err)
	}
	page := NewPage(story)
	page.Loaded = loaded
	page.Chrome = &Chrome{Settings: settings, HomeHref: "/", CurrentPath: "/landing-launch"}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := r.Page(io.Discard, page); err != nil {
				b.Error(err)
				return
			}
		}
	})
}
