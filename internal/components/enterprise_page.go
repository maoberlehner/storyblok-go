package components

import "storyblok-go-website/internal/storyblok"

func init() { register[EnterprisePage]("enterprise_page") }

type EnterprisePage struct {
	storyblok.Blok
	Body          Blocks          `json:"body"`
	MetaTitle     string          `json:"meta_title"`
	OGDescription string          `json:"og_description"`
	OGImage       storyblok.Asset `json:"og_image"`
}

func (p *EnterprisePage) Metadata() Metadata {
	return Metadata{Title: p.MetaTitle, Description: p.OGDescription, Image: p.OGImage}
}
