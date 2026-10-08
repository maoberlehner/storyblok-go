package components

import "storyblok-go-website/internal/schema"

func init() {
	register(schema.Definition[BlockMediaImage]{
		Name: "block-media-image", DisplayName: "Image", Category: schema.Media,
		Fields: []schema.Field{
			{Name: "image", Type: "asset", Label: "Image", Filetypes: []string{"images"}, Required: true,
				Description: "Set the alt text in the asset; leave it empty for decorative images."},
			{Name: "aspect_ratio", Type: "option", Label: "Aspect ratio", Default: "original", Options: []schema.Option{
				{Value: "original", Label: "Original"}, {Value: "16:9", Label: "16:9"},
				{Value: "4:3", Label: "4:3"}, {Value: "1:1", Label: "1:1"},
			}},
			{Name: "caption", Type: "text", Label: "Caption"},
		},
	}, blockMediaImage)
}
