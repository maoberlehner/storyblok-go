package components

import "storyblok-go-website/internal/storyblok"

type BlockContentHeadline struct {
	storyblok.Blok
	Text  string `json:"text"`
	Level string `json:"level"`
}
