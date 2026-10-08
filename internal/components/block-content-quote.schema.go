package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentQuote]{
		Name: "block-content-quote", DisplayName: "Quote", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "quote", Type: "textarea", Label: "Quote", Required: true},
			{Name: "name", Type: "text", Label: "Name", Required: true},
			{Name: "role", Type: "text", Label: "Role and company"},
		},
	}, blockContentQuote)
}
