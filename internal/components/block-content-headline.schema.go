package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentHeadline]{
		Name: "block-content-headline", DisplayName: "Headline", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "text", Type: "text", Label: "Text", Required: true},
			{Name: "level", Type: "option", Label: "Level", Default: "h2",
				Description: "Follow the document outline: h2 for sections, h3 and h4 below.",
				Options:     []schema.Option{{Value: "h2", Label: "Heading 2"}, {Value: "h3", Label: "Heading 3"}, {Value: "h4", Label: "Heading 4"}}},
		},
	})
}
