package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[SiteLink]{
		Name: "site-link", DisplayName: "Link", Category: schema.Site,
		Fields: []schema.Field{
			{Name: "label", Type: "text", Label: "Label", Required: true},
			{Name: "link", Type: "multilink", Label: "Link", Required: true},
		},
	})
}
