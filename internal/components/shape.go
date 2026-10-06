package components

import (
	"embed"
	"html/template"
	"strings"
)

// Decorative illustrations editors pick by name, such as background shapes
// and card decorations. They are drawn in currentColor. Add one by dropping
// its SVG into shapes/.
//
//go:embed shapes/*.svg
var shapeFS embed.FS

type Shape string

// SVG renders the shape. Unknown shapes render nothing.
func (s Shape) SVG() template.HTML {
	if s == "" || strings.ContainsAny(string(s), "./") {
		return ""
	}
	svg, err := shapeFS.ReadFile("shapes/" + string(s) + ".svg")
	if err != nil {
		return ""
	}
	return template.HTML(svg)
}
