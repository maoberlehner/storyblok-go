package components

import (
	"regexp"

	"storyblok-go-website/internal/storyblok"
)

func init() {
	register[BulletLists]("bullet_lists")
	register[BulletList]("bullet_list")
	register[BulletListItem]("bullet_list_item")
}

// BulletLists lays out bullet lists side by side.
type BulletLists struct {
	storyblok.Blok
	Items Blocks `json:"items"`
}

type BulletList struct {
	storyblok.Blok
	Headline          string          `json:"headline"`
	HeadlineIcon      string          `json:"headline_icon"`
	HeadlineAlignment string          `json:"headline_alignment"`
	AccentColor       storyblok.Color `json:"accent_color"`
	BulletPoints      Blocks          `json:"bullet_points"`
}

// DividerClass places the divider line above or below the headline.
func (b *BulletList) DividerClass() string {
	switch b.HeadlineAlignment {
	case "above_divider":
		return "bullet-list__headline-wrapper--divider-below"
	case "below_divider":
		return "bullet-list__headline-wrapper--divider-above"
	}
	return ""
}

func (b *BulletList) HeadlineIconURL() string { return lucideIconURL(b.HeadlineIcon) }

type BulletListItem struct {
	storyblok.Blok
	Icon string             `json:"icon"`
	Text storyblok.Richtext `json:"text"`
}

func (b *BulletListItem) IconURL() string { return lucideIconURL(b.Icon) }

const lucideIconsBaseURL = "https://cdn.jsdelivr.net/npm/lucide-static@1.52.0/icons/"

var lucideIconName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// lucideIconURL returns the URL of a Lucide icon, which icon option fields
// reference by name, or "" when name is empty or not a valid icon name.
func lucideIconURL(name string) string {
	if !lucideIconName.MatchString(name) {
		return ""
	}
	return lucideIconsBaseURL + name + ".svg"
}
