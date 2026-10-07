package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionTestimonials]{
		Name: "block-section-testimonials", DisplayName: "Testimonials section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "items", Type: "bloks", Label: "Items", Allow: schema.Content},
			sectionBackgroundField("default"),
		},
	})
}
