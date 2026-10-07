package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[SiteLinkGroup]{
		Name: "site-link-group", DisplayName: "Link group", Category: schema.Site,
		Fields: []schema.Field{
			{Name: "heading", Type: "text", Label: "Heading", Required: true},
			{Name: "links", Type: "bloks", Label: "Links", Components: []string{"site-link"}},
		},
	})
}
