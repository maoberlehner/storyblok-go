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

func TestAssetImage(t *testing.T) {
	photo := storyblok.Asset{Filename: "https://a.storyblok.com/f/1/1200x800/abc/photo.jpg", Focus: "10x20:30x40"}
	for _, tt := range []struct {
		opts storyblok.ImageOptions
		want string
	}{
		{storyblok.ImageOptions{Width: 600}, photo.Filename + "/m/600x0"},
		{storyblok.ImageOptions{Width: 600, Format: "avif"}, photo.Filename + "/m/600x0/filters:format(avif)"},
		{storyblok.ImageOptions{Width: 600, Height: 300}, photo.Filename + "/m/600x300/filters:focal(10x20:30x40)"},
		{storyblok.ImageOptions{Width: 600, Height: 300, Format: "avif"}, photo.Filename + "/m/600x300/filters:focal(10x20:30x40):format(avif)"},
	} {
		if got := photo.Image(tt.opts); got != tt.want {
			t.Errorf("Image(%+v) = %q, want %q", tt.opts, got, tt.want)
		}
	}
	logo := storyblok.Asset{Filename: "https://a.storyblok.com/f/1/41x41/abc/logo.svg"}
	if got := logo.Image(storyblok.ImageOptions{Width: 100}); got != logo.Filename {
		t.Errorf("Image() of SVG = %q, want original", got)
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
