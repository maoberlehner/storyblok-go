package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionFaq struct {
	storyblok.Blok
	Heading string `json:"heading"`
	Items   Blocks `json:"items"`
}
