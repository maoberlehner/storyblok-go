package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionCta]{
		Name: "block-section-cta", DisplayName: "Call to action section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "text", Type: "textarea", Label: "Text"},
			{Name: "link", Type: "multilink", Label: "Link", Required: true},
			{Name: "link_label", Type: "text", Label: "Link label", Required: true},
			sectionBackgroundField("accent"),
		},
	})
}
