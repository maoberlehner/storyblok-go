package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentStat]{
		Name: "block-content-stat", DisplayName: "Stat", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "value", Type: "text", Label: "Value", Description: "A short figure, e.g. 40%.", Required: true},
			{Name: "label", Type: "text", Label: "Label", Required: true},
		},
	})
}
