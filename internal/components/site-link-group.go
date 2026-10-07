package components

import "storyblok-go-website/internal/storyblok"

type SiteLinkGroup struct {
	storyblok.Blok
	Heading string `json:"heading"`
	Links   Blocks `json:"links"`
}
