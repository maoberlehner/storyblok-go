package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CardGrid]("card_grid") }

type CardGrid struct {
	storyblok.Blok
	Headline    storyblok.Richtext `json:"headline"`
	Subheadline string             `json:"subheadline"`
	Cards       Blocks             `json:"cards"`
	Alignment   string             `json:"alignment"`
}
