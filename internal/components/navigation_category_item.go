package components

import "storyblok-go-website/internal/storyblok"

func init() { register[NavigationCategoryItem]("navigation_category_item") }

type NavigationCategoryItem struct {
	storyblok.Blok
	Display string          `json:"display"`
	Link    storyblok.Link  `json:"link"`
	Badge   string          `json:"badge"`
	Text    string          `json:"text"`
	Icon    storyblok.Asset `json:"icon"`
}
