package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionStats struct {
	storyblok.Blok
	SectionStyle
	Heading string `json:"heading"`
	Items   Blocks `json:"items"`
}
