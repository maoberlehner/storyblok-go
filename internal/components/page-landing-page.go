package components

import "storyblok-go-website/internal/storyblok"

type PageLandingPage struct {
	storyblok.Blok
	Title    string `json:"title"`
	Sections Blocks `json:"sections"`
}

func (p *PageLandingPage) Metadata() Metadata { return Metadata{Title: p.Title} }
