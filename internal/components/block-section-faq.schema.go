package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionFaq]{
		Name: "block-section-faq", DisplayName: "FAQ section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "items", Type: "bloks", Label: "Items", Allow: schema.Content},
		},
	})
}
