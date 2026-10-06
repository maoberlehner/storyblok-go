package components

import "storyblok-go-website/internal/storyblok"

// Configuration is the global "config" story holding site-wide navigation.
type Configuration struct {
	storyblok.Blok
	GlobalAnnouncement storyblok.Richtext `json:"global_announcement"`
	Header             Blocks             `json:"new_header"`
	Footer             Blocks             `json:"new_footer"`
	FooterSocials      Blocks             `json:"new_footer_socials"`
}
