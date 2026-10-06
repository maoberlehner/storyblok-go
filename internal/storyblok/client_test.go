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
