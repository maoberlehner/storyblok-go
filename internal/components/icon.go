package components

import (
	"embed"
	"html/template"
	"io/fs"
	"path"
	"strings"
)

// Lucide icons (https://lucide.dev, ISC license), the icon set editors pick
// from in icon option fields. Add an icon by dropping its SVG into icons/.
//
//go:embed icons/*.svg
var iconFS embed.FS

var iconPaths = loadIcons()

func loadIcons() map[string]string {
	files, _ := fs.Glob(iconFS, "icons/*.svg")
	icons := make(map[string]string, len(files))
	for _, file := range files {
		svg, err := iconFS.ReadFile(file)
		if err != nil {
			continue
		}
		_, inner, ok := strings.Cut(string(svg), ">\n")
		if !ok {
			continue
		}
		// The first ">\n" closes the license comment; the next one the <svg> tag.
		if strings.HasPrefix(strings.TrimSpace(string(svg)), "<!--") {
			_, inner, _ = strings.Cut(inner, ">\n")
		}
		inner = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(inner), "</svg>"))
		icons[strings.TrimSuffix(path.Base(file), ".svg")] = inner
	}
	return icons
}

// Icon is the name of a Lucide icon.
type Icon string

// SVG renders the icon sized to the surrounding text. Unknown icons render
// nothing.
func (i Icon) SVG() template.HTML {
	paths, ok := iconPaths[string(i)]
	if !ok {
		return ""
	}
	return template.HTML(`<svg aria-hidden="true" viewBox="0 0 24 24" width="1.2em" height="1.2em" fill="none" ` +
		`stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`)
}
