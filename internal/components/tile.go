package components

import (
	"strconv"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[Tile]("tile") }

type Tile struct {
	storyblok.Blok
	Width                 string             `json:"width"`
	LayoutWidth50         string             `json:"layout_width_50"`
	LayoutWidth100        string             `json:"layout_width_100"`
	RelativeMaxImageWidth string             `json:"relative_max_image_width"`
	AlignImageToTileEdge  bool               `json:"align_image_to_tile_edge"`
	BackgroundColor       storyblok.Color    `json:"background_color"`
	Title                 storyblok.Richtext `json:"title"`
	Image                 storyblok.Asset    `json:"image"`
	Description           storyblok.Richtext `json:"description"`
	Link                  storyblok.Link     `json:"link"`
}

const defaultHalfTileLayout = "title_description_image"

func (t *Tile) IsFullWidth() bool { return t.Width == "100" }

// FullWidthLayout is "image_left" or "image_right".
func (t *Tile) FullWidthLayout() string {
	if t.LayoutWidth100 == "image_left" {
		return "image_left"
	}
	return "image_right"
}

// Parts returns "title", "description" and "image" in document order. Full
// width tiles place the image with CSS grid instead.
func (t *Tile) Parts() []string {
	layout := t.LayoutWidth50
	if t.IsFullWidth() || layout == "" {
		layout = defaultHalfTileLayout
	}
	return strings.Split(layout, "_")
}

// ImageEdge returns "top" or "bottom" when the image should bleed into the
// tile's padding at that edge, or "" when it stays inset.
func (t *Tile) ImageEdge() string {
	if !t.AlignImageToTileEdge || t.IsFullWidth() {
		return ""
	}
	parts := t.Parts()
	switch "image" {
	case parts[0]:
		return "top"
	case parts[len(parts)-1]:
		return "bottom"
	}
	return ""
}

// ImageWidth is the image's share of the tile width as a CSS percentage.
func (t *Tile) ImageWidth() string {
	percent, err := strconv.Atoi(t.RelativeMaxImageWidth)
	if err != nil || percent <= 0 || percent > 100 {
		percent = 100
	}
	return strconv.Itoa(percent) + "%"
}

func (t *Tile) ImageSizes() string {
	if t.IsFullWidth() {
		return "(min-width: 1280px) 700px, (min-width: 768px) 50vw, 100vw"
	}
	return "(min-width: 1280px) 550px, (min-width: 768px) 50vw, 100vw"
}
