package components

import "storyblok-go-website/internal/storyblok"

func init() { register[NavigationCategory]("navigation_category") }

// NavigationCategory groups dropdown entries under an optional headline.
type NavigationCategory struct {
	storyblok.Blok
	Headline        string         `json:"headline"`
	NavigationItems Blocks         `json:"navigation_items"`
	GroupLink       storyblok.Link `json:"group_link"`
	GroupLinkText   string         `json:"group_link_text"`
}
