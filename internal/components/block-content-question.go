package components

import "storyblok-go-website/internal/storyblok"

type BlockContentQuestion struct {
	storyblok.Blok
	Question string `json:"question"`
	Answer   string `json:"answer"`
}
