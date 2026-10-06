package components

import "storyblok-go-website/internal/storyblok"

func init() { register[EnterpriseSpacer]("enterprise_spacer") }

type EnterpriseSpacer struct {
	storyblok.Blok
	Height         string `json:"spacer_height"`
	HorizontalLine bool   `json:"horizontal_line"`
}

var spacerSizes = map[string]string{"30": "sm", "80": "md", "100": "base", "160": "xl"}

func (s *EnterpriseSpacer) Size() string {
	if size, ok := spacerSizes[s.Height]; ok {
		return size
	}
	return "md"
}
