package components

import "storyblok-go-website/internal/storyblok"

func init() { register[EnterpriseCta]("enterprise_cta") }

type EnterpriseCta struct {
	storyblok.Blok
	Text  string         `json:"text"`
	Link  storyblok.Link `json:"link"`
	Color string         `json:"color"`
}

// buttonVariants maps the legacy color options editors pick to button styles.
var buttonVariants = map[string]string{
	"button--hp-primary":   "primary",
	"button--hp-secondary": "secondary",
	"e-button--dark-blue":  "secondary",
	"button--light-yellow": "light-yellow",
}

func (c *EnterpriseCta) Variant() string {
	if v, ok := buttonVariants[c.Color]; ok {
		return v
	}
	return "primary"
}

// OpensNewWindow reports whether the link leaves the site, which storyblok.com
// opens in a new window.
func (c *EnterpriseCta) OpensNewWindow() bool {
	return c.Link.Target == "_blank" || c.Link.IsExternal()
}
