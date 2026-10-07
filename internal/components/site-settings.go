package components

import "storyblok-go-website/internal/storyblok"

// SiteSettings is the content of the "settings" story: navigation, footer,
// and defaults every page shares.
type SiteSettings struct {
	storyblok.Blok
	SiteName       string          `json:"site_name"`
	Navigation     Blocks          `json:"navigation"`
	CtaLink        storyblok.Link  `json:"cta_link"`
	CtaLabel       string          `json:"cta_label"`
	FooterColumns  Blocks          `json:"footer_columns"`
	LegalLinks     Blocks          `json:"legal_links"`
	Copyright      string          `json:"copyright"`
	DefaultOGImage storyblok.Asset `json:"default_og_image"`
}

// Button returns the header's call to action, or nil without a link and label.
func (s *SiteSettings) Button() *BaseButton {
	if s.CtaLink.IsZero() || s.CtaLabel == "" {
		return nil
	}
	return &BaseButton{Href: s.CtaLink.Href(), Label: s.CtaLabel}
}

// PreviewChrome shows the settings in the Visual Editor as header and footer.
func (s *SiteSettings) PreviewChrome() Chrome { return Chrome{Settings: s, HomeHref: "/"} }
