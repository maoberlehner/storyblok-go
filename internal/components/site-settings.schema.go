package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[SiteSettings]{
		Name: "site-settings", DisplayName: "Site settings", Category: schema.Site, ContentType: true,
		Fields: []schema.Field{
			{Name: "general", Type: "section", Label: "General", Fields: []schema.Field{
				{Name: "site_name", Type: "text", Label: "Site name", Required: true,
					Description: "Shown in the header and used for link previews and the web app manifest."},
			}},
			{Name: "header", Type: "section", Label: "Header", Fields: []schema.Field{
				{Name: "navigation", Type: "bloks", Label: "Navigation", Components: []string{"site-link"}},
				{Name: "cta_label", Type: "text", Label: "Call to action label"},
				{Name: "cta_link", Type: "multilink", Label: "Call to action link"},
			}},
			{Name: "footer", Type: "section", Label: "Footer", Fields: []schema.Field{
				{Name: "footer_columns", Type: "bloks", Label: "Link columns", Components: []string{"site-link-group"}},
				{Name: "legal_links", Type: "bloks", Label: "Legal links", Components: []string{"site-link"}},
				{Name: "copyright", Type: "text", Label: "Copyright"},
			}},
			{Name: "social", Type: "section", Label: "Social sharing", Fields: []schema.Field{
				{Name: "default_og_image", Type: "asset", Label: "Default share image", Filetypes: []string{"images"},
					Description: "Used for link previews of pages without their own image. Shown at 1200×630."},
			}},
		},
	}, siteSettings)
}
