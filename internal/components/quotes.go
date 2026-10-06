package components

import "storyblok-go-website/internal/storyblok"

func init() {
	register[Quotes]("quotes")
	register[QuotesItem]("quotes_item")
}

// maxStaticQuotes is the most quotes shown side by side; more scroll.
const maxStaticQuotes = 2

type Quotes struct {
	storyblok.Blok
	Quotes Blocks `json:"quotes"`
}

func (q *Quotes) Scrolls() bool { return len(q.Quotes) > maxStaticQuotes }

type QuotesItem struct {
	storyblok.Blok
	Headline           string                    `json:"headline"`
	HeadlineDecoration storyblok.Asset           `json:"headline_decoration"`
	Quote              storyblok.Relation[Quote] `json:"quote"`
}
