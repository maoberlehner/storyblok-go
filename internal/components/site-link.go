package components

import "storyblok-go-website/internal/storyblok"

type SiteLink struct {
	storyblok.Blok
	Link  storyblok.Link `json:"link"`
	Label string         `json:"label"`
}

func (l *SiteLink) IsZero() bool { return l.Link.IsZero() || l.Label == "" }
