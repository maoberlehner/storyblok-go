package components

import "storyblok-go-website/internal/storyblok"

func init() {
	register[CustomBoxesGrid]("custom_boxes_grid")
	register[CustomBoxesGridSmallBox]("custom_boxes_grid_small_box")
}

type CustomBoxesGrid struct {
	storyblok.Blok
	Boxes Blocks `json:"boxes"`
	// Columns is "2", "3" or "4"; three columns is the default layout.
	Columns        string `json:"columns"`
	ImagesPosition string `json:"images_position"`
}

func (g *CustomBoxesGrid) InlineImages() bool { return g.ImagesPosition == "inline" }

type CustomBoxesGridSmallBox struct {
	storyblok.Blok
	Image    storyblok.Asset    `json:"image"`
	Headline string             `json:"headline"`
	Content  storyblok.Richtext `json:"content"`
	Link     storyblok.Link     `json:"link"`
}
