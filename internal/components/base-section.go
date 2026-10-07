package components

import (
	"slices"

	"storyblok-go-website/internal/schema"
)

// sectionBackgrounds are the semantic backgrounds editors can choose. Each
// maps to global color properties in base-section.css, so a rebrand changes
// base.css only and editors can't pick inaccessible combinations.
var sectionBackgrounds = []schema.Option{
	{Value: "default", Label: "Default"},
	{Value: "muted", Label: "Muted"},
	{Value: "accent", Label: "Accent"},
}

// SectionStyle holds the presentation choices every section offers.
type SectionStyle struct {
	Background string `json:"background"`
}

// SectionBackground is the chosen background. Sections with another default
// than "default" override it.
func (s SectionStyle) SectionBackground() string { return s.Background }

type styledSection interface {
	SectionBackground() string
}

func sectionBackgroundField(defaultValue string) schema.Field {
	return schema.Field{Name: "background", Type: "option", Label: "Background", Options: sectionBackgrounds, Default: defaultValue}
}

// sectionClass returns the class list of a section's root element.
func sectionClass(block Block) string {
	class := block.Meta().Component + " base-section"
	if s, ok := block.(styledSection); ok {
		background := s.SectionBackground()
		isOption := func(o schema.Option) bool { return o.Value == background }
		if background != "default" && slices.ContainsFunc(sectionBackgrounds, isOption) {
			class += " base-section--" + background
		}
	}
	return class
}
