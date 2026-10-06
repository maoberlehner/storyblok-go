package components

import (
	"html/template"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[FooterSocial]("footer_social") }

type FooterSocial struct {
	storyblok.Blok
	Display string         `json:"display"`
	Link    storyblok.Link `json:"link"`
	// IconName is one of the social network names, e.g. "bluesky".
	IconName string `json:"icon"`
}

func (s *FooterSocial) Icon() template.HTML { return brandSVG("social-" + s.IconName) }
