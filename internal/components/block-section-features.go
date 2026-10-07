package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionFeatures struct {
	storyblok.Blok
	SectionStyle
	Heading string `json:"heading"`
	Text    string `json:"text"`
	Items   Blocks `json:"items"`
}
