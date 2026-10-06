package components

import "storyblok-go-website/internal/storyblok"

func init() {
	register[FactsWithImages]("facts_with_images")
	register[FactWithImage]("fact_with_image")
}

type FactsWithImages struct {
	storyblok.Blok
	Facts Blocks `json:"facts"`
}

// FactWithImage is a key figure, such as a customer metric next to its logo.
type FactWithImage struct {
	storyblok.Blok
	Image storyblok.Asset `json:"image"`
	Value string          `json:"value"`
	Key   string          `json:"key"`
}
