package components

import "storyblok-go-website/internal/storyblok"

type PageArticle struct {
	storyblok.Blok
	PageMeta
	Sections Blocks `json:"sections"`
}

func (p *PageArticle) Metadata() Metadata {
	m := p.PageMeta.Metadata()
	m.Type = "article"
	return m
}

func (p *PageArticle) SectionBlocks() Blocks { return p.Sections }
