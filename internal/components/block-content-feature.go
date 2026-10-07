package components

import "storyblok-go-website/internal/storyblok"

type BlockContentFeature struct {
	storyblok.Blok
	Title string `json:"title"`
	Text  string `json:"text"`
}
