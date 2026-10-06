package components

import "storyblok-go-website/internal/storyblok"

func init() { register[CaseStudyBigCard]("case_study_big_card") }

type CaseStudyBigCard struct {
	storyblok.Blok
	Title                string          `json:"title"`
	Description          string          `json:"description"`
	Logo                 storyblok.Asset `json:"logo"`
	Image                storyblok.Asset `json:"image"`
	Metrics              Blocks          `json:"metrics"`
	CTA                  Blocks          `json:"cta"`
	ShowCornerDecoration bool            `json:"show_corner_decoration"`
}
