package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockSectionArticles]{
		Name: "block-section-articles", DisplayName: "Articles section", Category: schema.Section,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "folder", Type: "text", Label: "Folder", Description: "Full slug of the folder that holds the articles, e.g. articles.", Required: true},
			sectionBackgroundField("default"),
		},
	}, blockSectionArticles)
}
