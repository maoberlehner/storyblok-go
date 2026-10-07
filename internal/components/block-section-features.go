package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionFeatures struct {
	storyblok.Blok
	Heading string `json:"heading"`
	Text    string `json:"text"`
	Items   Blocks `json:"items"`
}
