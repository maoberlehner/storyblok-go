package components

import "storyblok-go-website/internal/storyblok"

type PageLandingPage struct {
	storyblok.Blok
	Title       string `json:"title"`
	Description string `json:"description"`
	Sections    Blocks `json:"sections"`
}

func (p *PageLandingPage) Metadata() Metadata {
	return Metadata{Title: p.Title, Description: p.Description}
}

func (p *PageLandingPage) SectionBlocks() Blocks { return p.Sections }
