package components

// Chrome is the site-wide header and footer around a page.
type Chrome struct {
	Settings *SiteSettings
	// HomeHref links the site name to the home page of the page's language.
	HomeHref string
	// CurrentPath marks the navigation link to the current page.
	CurrentPath string
	// Languages lists the page in each language, for the language switcher.
	Languages []LanguageLink
}

type LanguageLink struct {
	Lang, Label, Href string
	Current           bool
}

type NavLink struct {
	Block       Block
	Href, Label string
	Current     bool
}

// NavLinks are the header navigation's complete links.
func (c Chrome) NavLinks() []NavLink {
	var links []NavLink
	for _, block := range c.Settings.Navigation {
		link, ok := block.(*SiteLink)
		if !ok || link.IsZero() {
			continue
		}
		href := link.Link.Href()
		links = append(links, NavLink{Block: link, Href: href, Label: link.Label, Current: href == c.CurrentPath})
	}
	return links
}
