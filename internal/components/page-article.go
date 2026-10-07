package components

import "storyblok-go-website/internal/storyblok"

type PageArticle struct {
	storyblok.Blok
	Title       string `json:"title"`
	Description string `json:"description"`
	Sections    Blocks `json:"sections"`
}

func (p *PageArticle) Metadata() Metadata {
	return Metadata{Title: p.Title, Description: p.Description}
}

func (p *PageArticle) SectionBlocks() Blocks { return p.Sections }
