package components

import "storyblok-go-website/internal/storyblok"

func init() { register[ImageWithDecorations]("image_with_decorations") }

type ImageWithDecorations struct {
	storyblok.Blok
	Image       storyblok.Asset `json:"image"`
	Decorations []string        `json:"decorations"`
	Rotate      bool            `json:"rotate"`
}
