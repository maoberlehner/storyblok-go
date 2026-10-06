package components

import (
	"embed"
	"html/template"
)

//go:embed brand/*.svg
var brandFS embed.FS

// brandSVG returns the inline SVG markup of brand/<name>.svg, or nothing if
// the file does not exist, so unknown icon options in content render no markup.
func brandSVG(name string) template.HTML {
	svg, err := brandFS.ReadFile("brand/" + name + ".svg")
	if err != nil {
		return ""
	}
	return template.HTML(svg)
}
