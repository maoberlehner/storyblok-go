package components

import "storyblok-go-website/internal/storyblok"

// Quote is the content of a quote story referenced by quote relation fields.
type Quote struct {
	FullName string          `json:"full_name"`
	Role     string          `json:"role"`
	Text     string          `json:"text"`
	Image    storyblok.Asset `json:"image"`
}
