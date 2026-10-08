package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionHero]{
		Name: "block-section-hero", DisplayName: "Hero section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "text", Type: "textarea", Label: "Text"},
			{Name: "link", Type: "multilink", Label: "Call to action link"},
			{Name: "link_label", Type: "text", Label: "Call to action label"},
			{Name: "media", Type: "bloks", Label: "Media", Allow: schema.Media, Max: 1},
			sectionBackgroundField("default"),
		},
	}, blockSectionHero)
}
