package components

import (
	"cmp"

	"storyblok-go-website/internal/storyblok"
)

type BlockSectionCta struct {
	storyblok.Blok
	SectionStyle
	Heading   string         `json:"heading"`
	Text      string         `json:"text"`
	Link      storyblok.Link `json:"link"`
	LinkLabel string         `json:"link_label"`
}

// Button returns the call to action, or nil without a link and label.
func (c *BlockSectionCta) Button() *BaseButton {
	if c.Link.IsZero() || c.LinkLabel == "" {
		return nil
	}
	return &BaseButton{Href: c.Link.Href(), Label: c.LinkLabel}
}

// SectionBackground defaults to accent: a call to action stands out unless the
// editor chooses otherwise.
func (c *BlockSectionCta) SectionBackground() string { return cmp.Or(c.Background, "accent") }
