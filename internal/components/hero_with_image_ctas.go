package components

import (
	"embed"
	"html/template"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[HeroWithImageCtas]("hero_with_image_ctas") }

//go:embed hero_svg/*.svg
var heroSVGs embed.FS

type HeroWithImageCtas struct {
	storyblok.Blok
	Text        storyblok.Richtext        `json:"text"`
	Image       storyblok.Asset           `json:"image"`
	Quote       storyblok.Relation[Quote] `json:"quote"`
	Shape       string                    `json:"shape"`
	ShapeColour storyblok.Color           `json:"shape_colour"`
	Ctas        Blocks                    `json:"ctas"`
	Decorations []string                  `json:"decorations"`
}

// ShapeSVG returns the background blob for the "shapes" datasource value, or
// nothing for shapes without artwork.
func (h *HeroWithImageCtas) ShapeSVG() template.HTML {
	return heroSVG("shape-" + h.Shape)
}

// DecorationSVG returns the doodle drawn around the image. Top and bottom
// "attention" marks share artwork and are rotated with CSS.
func (h *HeroWithImageCtas) DecorationSVG(name string) template.HTML {
	if strings.HasPrefix(name, "attention_") {
		name = "attention"
	}
	return heroSVG("decoration-" + name)
}

func heroSVG(name string) template.HTML {
	if strings.ContainsAny(name, "/.") {
		return ""
	}
	svg, err := heroSVGs.ReadFile("hero_svg/" + name + ".svg")
	if err != nil {
		return ""
	}
	return template.HTML(svg)
}
