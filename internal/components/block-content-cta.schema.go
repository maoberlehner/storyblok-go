package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockContentCta]{
		Name: "block-content-cta", DisplayName: "Call to action", Category: schema.Content,
		Fields: []schema.Field{
			{Name: "link", Type: "multilink", Label: "Link", Required: true},
			{Name: "label", Type: "text", Label: "Label", Required: true},
			{Name: "variant", Type: "option", Label: "Variant", Default: "primary",
				Options: []schema.Option{{Value: "primary", Label: "Primary"}, {Value: "secondary", Label: "Secondary"}}},
		},
	}, blockContentCta)
}
