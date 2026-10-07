package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionContent]{
		Name: "block-section-content", DisplayName: "Content section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "content", Type: "bloks", Label: "Content", Allow: schema.Content},
			{Name: "media", Type: "bloks", Label: "Media", Allow: schema.Media, Max: 1},
			{Name: "media_position", Type: "option", Label: "Media position", Default: "end",
				Description: "Where media appears on wide screens. Small screens show it below the content.",
				Options:     []schema.Option{{Value: "end", Label: "End"}, {Value: "start", Label: "Start"}}},
			sectionBackgroundField("default"),
		},
	})
}
