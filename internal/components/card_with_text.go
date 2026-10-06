package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CardWithText]("card_with_text") }

type CardWithText struct {
	storyblok.Blok
	Decoration  Shape              `json:"decoration"`
	Title       string             `json:"title"`
	Description storyblok.Richtext `json:"description"`
	Link        storyblok.Link     `json:"link"`
	AccentColor storyblok.Color    `json:"accent_color"`
}
