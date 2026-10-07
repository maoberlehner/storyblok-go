package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[PageLandingPage]{
		Name: "page-landing-page", DisplayName: "Landing page", Category: schema.Page,
		Fields: append([]schema.Field{
			{Name: "title", Type: "text", Label: "Title", Description: "The page's main heading and document title.", Required: true},
			{Name: "description", Type: "textarea", Label: "Description", Description: "Shown in search results and link previews.", Required: true},
			{Name: "sections", Type: "bloks", Label: "Sections", Allow: schema.Section},
		}, schema.PageMetaGroups()...),
	})
}
