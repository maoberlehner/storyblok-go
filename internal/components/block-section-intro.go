package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionIntro struct {
	storyblok.Blok
	Heading string `json:"heading"`
	Text    string `json:"text"`
}
