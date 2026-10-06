package components

import "storyblok-go-website/internal/storyblok"

func init() { register[SectionWithQuoteCard]("section_with_quote_card") }

type SectionWithQuoteCard struct {
	storyblok.Blok
	Description     storyblok.Richtext        `json:"description"`
	QuoteTitle      string                    `json:"quote_title"`
	Quote           storyblok.Relation[Quote] `json:"quote"`
	Decoration      string                    `json:"decoration"`
	BackgroundColor storyblok.Color           `json:"background_color"`
	CTAs            Blocks                    `json:"ctas"`
}
