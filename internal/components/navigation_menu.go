package components

import "storyblok-go-website/internal/storyblok"

func init() { register[NavigationMenu]("navigation_menu") }

// NavigationMenu is a top-level header entry with a dropdown.
type NavigationMenu struct {
	storyblok.Blok
	Display  string         `json:"display"`
	Link     storyblok.Link `json:"link"`
	NavItems Blocks         `json:"nav_items"`
}
