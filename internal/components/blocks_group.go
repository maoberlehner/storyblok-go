package components

import (
	"html/template"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[BlocksGroup]("blocks_group") }

type BlocksGroup struct {
	storyblok.Blok
	BackgroundColor          storyblok.Color `json:"background_color_from_plugin"`
	BackgroundImage          Shape           `json:"background_image"`
	BackgroundImageAlignment string          `json:"background_image_alignment"`
	ShapeVariant             string          `json:"shape_variant"`
	ContentWidth             string          `json:"content_width"`
	Spacing                  string          `json:"spacing"`
	Blocks                   Blocks          `json:"blocks"`
}

func (g *BlocksGroup) SpacingClass() string {
	if g.Spacing == "large" {
		return "large"
	}
	return "small"
}

func (g *BlocksGroup) ContentWidthClass() string {
	return strings.TrimSpace(g.ContentWidth)
}

// BackgroundShape is only visible on a colored background, so it is omitted
// without one.
func (g *BlocksGroup) BackgroundShape() template.HTML {
	if g.BackgroundColor.Value == "" {
		return ""
	}
	return g.BackgroundImage.SVG()
}

func (g *BlocksGroup) BackgroundAlignment() string {
	switch g.BackgroundImageAlignment {
	case "top-left", "top-right", "bottom-left":
		return g.BackgroundImageAlignment
	}
	return "bottom-right"
}
