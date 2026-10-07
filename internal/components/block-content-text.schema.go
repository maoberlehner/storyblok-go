package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentText]{
		Name: "block-content-text", DisplayName: "Text", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "text", Type: "markdown", Label: "Text", Required: true,
				Description: "Markdown. Headings start at level 2; HTML is not rendered."},
		},
	})
}
