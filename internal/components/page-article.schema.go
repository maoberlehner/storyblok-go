package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[PageArticle]{
		Name: "page-article", DisplayName: "Article", Category: schema.Page,
		Fields: append([]schema.Field{
			{Name: "title", Type: "text", Label: "Title", Description: "The article's heading and document title.", Required: true},
			{Name: "description", Type: "textarea", Label: "Description", Description: "Shown below the title, in article listings, search results, and link previews.", Required: true},
			{Name: "sections", Type: "bloks", Label: "Sections", Allow: schema.Section},
		}, schema.PageMetaGroups()...),
	})
}
