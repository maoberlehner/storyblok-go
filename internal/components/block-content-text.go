package components

import "storyblok-go-website/internal/storyblok"

type BlockContentText struct {
	storyblok.Blok
	Text storyblok.Markdown `json:"text"`
}
