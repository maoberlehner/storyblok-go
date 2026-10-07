package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentQuestion]{
		Name: "block-content-question", DisplayName: "Question", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "question", Type: "text", Label: "Question", Required: true},
			{Name: "answer", Type: "textarea", Label: "Answer", Required: true},
		},
	})
}
