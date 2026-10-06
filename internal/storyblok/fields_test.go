package storyblok_test

import (
	"encoding/json/v2"
	"testing"

	"storyblok-go-website/internal/storyblok"
)

func TestLinkHref(t *testing.T) {
	tests := []struct {
		name string
		link storyblok.Link
		want string
	}{
		{"story", storyblok.Link{LinkType: "story", CachedURL: "fs/enterprise-contact"}, "/fs/enterprise-contact"},
		{"home story", storyblok.Link{LinkType: "story", CachedURL: "home"}, "/"},
		{"story with anchor", storyblok.Link{LinkType: "story", CachedURL: "pricing/", Anchor: "plans"}, "/pricing#plans"},
		{"url", storyblok.Link{LinkType: "url", URL: "https://example.com"}, "https://example.com"},
		{"email", storyblok.Link{LinkType: "email", Email: "hi@example.com"}, "mailto:hi@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.link.Href(); got != tt.want {
				t.Errorf("Href() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAssetResize(t *testing.T) {
	photo := storyblok.Asset{Filename: "https://a.storyblok.com/f/1/1200x800/abc/photo.jpg"}
	if got, want := photo.Resize(600, 0), photo.Filename+"/m/600x0"; got != want {
		t.Errorf("Resize() = %q, want %q", got, want)
	}
	if got, want := photo.SrcSet(600, 1200, 2400), photo.Filename+"/m/600x0 600w, "+photo.Filename+"/m/1200x0 1200w"; got != want {
		t.Errorf("SrcSet() = %q, want %q", got, want)
	}

	logo := storyblok.Asset{Filename: "https://a.storyblok.com/f/1/41x41/abc/logo.svg"}
	if got := logo.Resize(100, 0); got != logo.Filename {
		t.Errorf("Resize() of SVG = %q, want original", got)
	}
}

func TestEmptyFieldsDecodeToZeroValues(t *testing.T) {
	var fields struct {
		Image storyblok.Asset    `json:"image"`
		Link  storyblok.Link     `json:"link"`
		Color storyblok.Color    `json:"color"`
		Text  storyblok.Richtext `json:"text"`
	}
	if err := json.Unmarshal([]byte(`{"image":null,"link":"","color":"","text":""}`), &fields); err != nil {
		t.Fatal(err)
	}
	if !fields.Image.IsZero() || !fields.Link.IsZero() || fields.Color.Value != "" || !fields.Text.IsEmpty() {
		t.Errorf("got %+v, want zero values", fields)
	}
}
