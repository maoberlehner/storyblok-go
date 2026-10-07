package components

import "storyblok-go-website/internal/storyblok"

type BlockContentStat struct {
	storyblok.Blok
	Value string `json:"value"`
	Label string `json:"label"`
}
