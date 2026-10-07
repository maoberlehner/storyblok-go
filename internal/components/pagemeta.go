package components

import "storyblok-go-website/internal/storyblok"

// PageMeta holds the fields every page has for search engines and link
// previews. Pages embed it; their schemas add schema.PageMetaGroups().
type PageMeta struct {
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	SEOTitle       string          `json:"seo_title"`
	SEODescription string          `json:"seo_description"`
	OGTitle        string          `json:"og_title"`
	OGDescription  string          `json:"og_description"`
	OGImage        storyblok.Asset `json:"og_image"`
}

func (m PageMeta) Metadata() Metadata {
	return Metadata{
		Title: m.Title, Description: m.Description,
		SEOTitle: m.SEOTitle, SEODescription: m.SEODescription,
		OGTitle: m.OGTitle, OGDescription: m.OGDescription, OGImage: m.OGImage,
	}
}
