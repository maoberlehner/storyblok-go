package components

import "storyblok-go-website/internal/storyblok"

type PageLandingPage struct {
	storyblok.Blok
	PageMeta
	Sections Blocks `json:"sections"`
}

func (p *PageLandingPage) SectionBlocks() Blocks { return p.Sections }
