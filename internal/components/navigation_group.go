package components

import "storyblok-go-website/internal/storyblok"

func init() { register[NavigationGroup]("navigation_group") }

// NavigationGroup is a footer link column.
type NavigationGroup struct {
	storyblok.Blok
	GroupName string `json:"group_name"`
	NavItems  Blocks `json:"navitems"`
}
