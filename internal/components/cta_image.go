package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CtaImage]("cta_image") }

type CtaImage struct {
	storyblok.Blok
	TextColor            string             `json:"text_color"`
	BackgroundColorLight storyblok.Color    `json:"background_color_light"`
	BackgroundColorDark  storyblok.Color    `json:"background_color_dark"`
	Image                storyblok.Asset    `json:"image"`
	Content              storyblok.Richtext `json:"content"`
	Cta                  Blocks             `json:"cta"`
}

// HasDarkText reports whether the banner uses dark text on a light
// background. Storyblok's schema defaults to white text.
func (c *CtaImage) HasDarkText() bool { return c.TextColor == "dark" }
