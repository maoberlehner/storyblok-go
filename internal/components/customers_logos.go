package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CustomersLogos]("customers_logos") }

// marqueeCopies is how often the logo list repeats so the scrolling track
// never shows a gap; the animation shifts the track by one copy.
const marqueeCopies = 3

type CustomersLogos struct {
	storyblok.Blok
	Headline string `json:"headline"`
	Logos    Blocks `json:"logos_list"`
	CTA      Blocks `json:"cta"`
}

func (c *CustomersLogos) LogoItems() []*Logo {
	return blocksOfType[*Logo](c.Logos)
}

// HiddenCopies is the number of decorative duplicates of the logo list.
func (c *CustomersLogos) HiddenCopies() int { return marqueeCopies - 1 }

func blocksOfType[T Block](blocks Blocks) []T {
	var matches []T
	for _, b := range blocks {
		if t, ok := b.(T); ok {
			matches = append(matches, t)
		}
	}
	return matches
}
