package components

import "storyblok-go-website/internal/storyblok"

func init() { register[FooterNavigationItem]("footer_navigation_item") }

type FooterNavigationItem struct {
	storyblok.Blok
	Display string         `json:"display"`
	Link    storyblok.Link `json:"link"`
	Badge   string         `json:"badge"`
}
