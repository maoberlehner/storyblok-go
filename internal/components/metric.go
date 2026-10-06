package components

import "storyblok-go-website/internal/storyblok"

func init() { register[Metric]("metric") }

type Metric struct {
	storyblok.Blok
	Label string `json:"label"`
	Value string `json:"value"`
}
