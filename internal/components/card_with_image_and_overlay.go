package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CardWithImageAndOverlay]("card_with_image_and_overlay") }

type CardWithImageAndOverlay struct {
	storyblok.Blok
	Title string             `json:"title"`
	Text  storyblok.Richtext `json:"text"`
	Image storyblok.Asset    `json:"image"`
	Link  storyblok.Link     `json:"link"`
	// Logo is placed over the image, so it is usually a white SVG.
	Logo storyblok.Asset `json:"logo"`
}
