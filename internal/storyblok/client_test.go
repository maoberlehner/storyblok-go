package storyblok_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storyblok-go-website/internal/storyblok"
)

func TestClientStory(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stories/home":
			if r.URL.Query().Get("version") != "draft" || r.URL.Query().Get("token") != "secret" {
				t.Errorf("unexpected query %v", r.URL.Query())
			}
			w.Write([]byte(`{
				"story": {"name": "Home", "content": {"component": "page", "body": [
					{"component": "quote_card", "quote": "q-1"}
				]}},
				"rels": [{"uuid": "q-1", "name": "Ada", "content": {"text": "Hello"}}]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	client := storyblok.NewClient(api.URL, "secret")

	t.Run("inlines resolved relations", func(t *testing.T) {
		raw, err := client.Story(t.Context(), "home", storyblok.StoryOptions{
			Version:          storyblok.Draft,
			ResolveRelations: []string{"quote_card.quote"},
		})
		if err != nil {
			t.Fatal(err)
		}
		var story struct {
			Content struct {
				Body []struct {
					Quote storyblok.Relation[struct {
						Text string `json:"text"`
					}] `json:"quote"`
				} `json:"body"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &story); err != nil {
			t.Fatal(err)
		}
		quote := story.Content.Body[0].Quote.Story
		if quote == nil || quote.Name != "Ada" || quote.Content.Text != "Hello" {
			t.Errorf("quote = %+v, want resolved story", quote)
		}
	})

	t.Run("reports missing stories", func(t *testing.T) {
		_, err := client.Story(t.Context(), "missing", storyblok.StoryOptions{Version: storyblok.Published})
		if !errors.Is(err, storyblok.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestClientStoryErrorsDoNotLeakToken(t *testing.T) {
	client := storyblok.NewClient("http://127.0.0.1:1", "secret-token")
	_, err := client.Story(t.Context(), "home", storyblok.StoryOptions{Version: storyblok.Published})
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Errorf("err = %v, want error without the token", err)
	}
}

func TestClientStories(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stories" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		for key, want := range map[string]string{
			"starts_with": "articles/", "content_type": "page-article", "page": "2", "per_page": "6",
			"sort_by": "first_published_at:desc", "excluding_fields": "sections", "version": "published",
		} {
			if got := q.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Total", "14")
		w.Write([]byte(`{"stories": [{"name": "Seven", "full_slug": "articles/seven"}], "cv": 3}`))
	}))
	defer api.Close()
	client := storyblok.NewClient(api.URL, "secret")

	list, err := client.Stories(t.Context(), storyblok.StoriesOptions{
		StartsWith: "articles/", ContentType: "page-article", Page: 2, PerPage: 6,
		SortBy: "first_published_at:desc", ExcludingFields: []string{"sections"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var stories []storyblok.Story[struct{}]
	if err := json.Unmarshal(list.Stories, &stories); err != nil {
		t.Fatal(err)
	}
	if list.Total != 14 || len(stories) != 1 || stories[0].FullSlug != "articles/seven" {
		t.Errorf("list = %+v, stories = %+v", list, stories)
	}
}

func TestClientRequestsFieldLevelTranslations(t *testing.T) {
	var languages []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		languages = append(languages, r.URL.Query().Get("language"))
		if r.URL.Path == "/stories" {
			w.Header().Set("Total", "0")
			w.Write([]byte(`{"stories": []}`))
			return
		}
		w.Write([]byte(`{"story": {"name": "Home", "content": {}}}`))
	}))
	defer api.Close()
	client := storyblok.NewClient(api.URL, "secret")
	opts := storyblok.StoryOptions{Version: storyblok.Draft}
	if _, err := client.Story(t.Context(), "home", opts); err != nil {
		t.Fatal(err)
	}
	opts.Language = "de"
	if _, err := client.Story(t.Context(), "home", opts); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Stories(t.Context(), storyblok.StoriesOptions{Version: storyblok.Draft, Language: "de"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(languages, ",") != ",de,de" {
		t.Errorf("language parameters = %q", languages)
	}
}
