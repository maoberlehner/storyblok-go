package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionIntro]{
		Name: "block-section-intro", DisplayName: "Intro section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "text", Type: "textarea", Label: "Text"},
			sectionBackgroundField("default"),
		},
	})
}
