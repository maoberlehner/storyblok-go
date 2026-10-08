package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionFeatures]{
		Name: "block-section-features", DisplayName: "Features section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "text", Type: "textarea", Label: "Text"},
			{Name: "items", Type: "bloks", Label: "Items", Allow: schema.Content},
			sectionBackgroundField("default"),
		},
	}, blockSectionFeatures)
}
