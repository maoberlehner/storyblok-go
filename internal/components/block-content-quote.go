package components

import "storyblok-go-website/internal/storyblok"

type BlockContentQuote struct {
	storyblok.Blok
	Quote string `json:"quote"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}
