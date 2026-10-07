package components

import "storyblok-go-website/internal/storyblok"

type BlockContentCta struct {
	storyblok.Blok
	Link    storyblok.Link `json:"link"`
	Label   string         `json:"label"`
	Variant string         `json:"variant"`
}

// Button returns the call to action, or nil without a link and label.
func (c *BlockContentCta) Button() *BaseButton {
	if c.Link.IsZero() || c.Label == "" {
		return nil
	}
	return &BaseButton{Href: c.Link.Href(), Label: c.Label, Variant: c.Variant}
}
