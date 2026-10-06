package components

import "storyblok-go-website/internal/storyblok"

func init() { register[Logo]("logo") }

type Logo struct {
	storyblok.Blok
	Logo storyblok.Asset `json:"logo"`
	Link storyblok.Link  `json:"link"`
}

// WidthAt returns the logo's width when scaled to height, keeping its aspect
// ratio. Without known dimensions the logo is treated as square.
func (l *Logo) WidthAt(height int) int {
	w, h := l.Logo.Size()
	if w == 0 || h == 0 {
		return height
	}
	return (w*height + h/2) / h
}
