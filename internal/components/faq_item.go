package components

import "storyblok-go-website/internal/storyblok"

func init() { register[FaqItem]("faq_item") }

type FaqItem struct {
	storyblok.Blok
	Question string             `json:"question"`
	Answer   storyblok.Richtext `json:"answer"`
	// Number is set by the enclosing FAQ list.
	Number string `json:"-"`
}
