package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[PageLandingPage]{
		Name: "page-landing-page", DisplayName: "Landing page", Category: schema.Page,
		Fields: []schema.Field{
			{Name: "title", Type: "text", Label: "Page title", Required: true},
			{Name: "sections", Type: "bloks", Label: "Sections", Allow: schema.Section},
		},
	})
}
