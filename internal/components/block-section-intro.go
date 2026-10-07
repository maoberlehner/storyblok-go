package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionIntro struct {
	storyblok.Blok
	SectionStyle
	Heading string `json:"heading"`
	Text    string `json:"text"`
}
