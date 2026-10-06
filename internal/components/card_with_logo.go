package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CardWithLogo]("card_with_logo") }

type CardWithLogo struct {
	storyblok.Blok
	Title           string             `json:"title"`
	Text            storyblok.Richtext `json:"text"`
	Logo            storyblok.Asset    `json:"logo"`
	Link            storyblok.Link     `json:"link"`
	BackgroundColor string             `json:"custom_background_color"`
}
