package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionFaq struct {
	storyblok.Blok
	SectionStyle
	Heading string `json:"heading"`
	Items   Blocks `json:"items"`
}
