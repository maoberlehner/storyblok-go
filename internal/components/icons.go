package components

import (
	"embed"
	"html/template"
)

//go:embed icons/*.svg
var iconFS embed.FS

// icon returns the inline SVG markup of icons/<name>.svg, or nothing if the
// icon does not exist, so unknown icon options in content render no markup.
func icon(name string) template.HTML {
	svg, err := iconFS.ReadFile("icons/" + name + ".svg")
	if err != nil {
		return ""
	}
	return template.HTML(svg)
}
