package components

import "storyblok-go-website/internal/storyblok"

func init() {
	register[RichtextHeadingWithDescriptionAndCtas]("richtext_heading_with_description_and_ctas")
}

type RichtextHeadingWithDescriptionAndCtas struct {
	storyblok.Blok
	Eyebrow               string             `json:"eyebrow"`
	EyebrowIcon           Icon               `json:"eyebrow_icon"`
	EyebrowAccentColor    storyblok.Color    `json:"eyebrow_accent_color"`
	Headline              storyblok.Richtext `json:"headline"`
	Description           storyblok.Richtext `json:"description"`
	Ctas                  Blocks             `json:"ctas"`
	SmallPrintInlineImage storyblok.Asset    `json:"small_print_inline_image"`
	SmallPrintText        string             `json:"small_print_text"`
	Layout                string             `json:"layout"`
	HeadlineSize          string             `json:"headline_size"`
	Alignment             string             `json:"alignment"`
}

func (h *RichtextHeadingWithDescriptionAndCtas) LayoutClass() string {
	if h.Layout == "row" {
		return "row"
	}
	return "column"
}

func (h *RichtextHeadingWithDescriptionAndCtas) IsCentered() bool {
	return h.Alignment != "left"
}

// IsLarge marks the page's main headline, rendered as h1.
func (h *RichtextHeadingWithDescriptionAndCtas) IsLarge() bool {
	return h.HeadlineSize == "large"
}

func (h *RichtextHeadingWithDescriptionAndCtas) HasSmallPrint() bool {
	return h.SmallPrintText != "" || !h.SmallPrintInlineImage.IsZero()
}
