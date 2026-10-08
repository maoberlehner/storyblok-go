package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionContact]{
		Name: "block-section-contact", DisplayName: "Contact section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "text", Type: "textarea", Label: "Text"},
			{Name: "success_message", Type: "textarea", Label: "Success message", Description: "Shown after the message was sent."},
			sectionBackgroundField("default"),
		},
	}, blockSectionContact)
}
