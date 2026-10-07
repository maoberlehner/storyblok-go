package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionHero struct {
	storyblok.Blok
	Heading   string         `json:"heading"`
	Text      string         `json:"text"`
	Link      storyblok.Link `json:"link"`
	LinkLabel string         `json:"link_label"`
}

// Button returns the call to action, or nil without a link and label.
func (h *BlockSectionHero) Button() *BaseButton {
	if h.Link.IsZero() || h.LinkLabel == "" {
		return nil
	}
	return &BaseButton{Href: h.Link.Href(), Label: h.LinkLabel}
}
