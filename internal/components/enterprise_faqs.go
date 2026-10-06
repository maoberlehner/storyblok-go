package components

import (
	"fmt"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[EnterpriseFaqs]("enterprise_faqs") }

type EnterpriseFaqs struct {
	storyblok.Blok
	Headline string             `json:"headline"`
	Text     storyblok.Richtext `json:"text"`
	Image    storyblok.Asset    `json:"image"`
	Faqs     Blocks             `json:"faqs"`
}

// NumberedFaqs returns the FAQ items labeled "01", "02", … in list order.
func (f *EnterpriseFaqs) NumberedFaqs() []Block {
	numbered := make([]Block, 0, len(f.Faqs))
	for _, block := range f.Faqs {
		if item, ok := block.(*FaqItem); ok {
			labeled := *item
			labeled.Number = fmt.Sprintf("%02d", len(numbered)+1)
			block = &labeled
		}
		numbered = append(numbered, block)
	}
	return numbered
}
