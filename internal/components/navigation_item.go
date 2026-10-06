package components

import "storyblok-go-website/internal/storyblok"

func init() { register[NavigationItem]("navigation_item") }

// NavigationItem is a top-level header entry without a dropdown.
type NavigationItem struct {
	storyblok.Blok
	Display string         `json:"display"`
	Link    storyblok.Link `json:"link"`
}
