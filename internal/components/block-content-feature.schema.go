package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentFeature]{
		Name: "block-content-feature", DisplayName: "Feature", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "title", Type: "text", Label: "Title", Required: true},
			{Name: "text", Type: "textarea", Label: "Text"},
		},
	})
}
