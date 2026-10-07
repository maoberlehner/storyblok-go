package server

import (
	"encoding/json/v2"
	"encoding/xml"
	"net/http"
	"slices"
	"strings"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/storyblok"
)

// sitemapPageSize is the Content Delivery API's maximum.
const sitemapPageSize = 100

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	XHTML   string       `xml:"xmlns:xhtml,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string        `xml:"loc"`
	LastMod    string        `xml:"lastmod,omitempty"`
	Alternates []sitemapLink `xml:"xhtml:link"`
}

type sitemapLink struct {
	Rel      string `xml:"rel,attr"`
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

// serveSitemap lists every published story with a page component. Search
// engines can't follow "load more" forms, so this is how they find articles
// beyond the first page of a listing.
func (s *Server) serveSitemap(w http.ResponseWriter, r *http.Request) {
	if s.siteURL == "" {
		http.NotFound(w, r)
		return
	}
	var urls []sitemapURL
	for page, listed := 1, 0; ; page++ {
		list, err := s.content.Stories(r.Context(), storyblok.StoriesOptions{
			Page: page, PerPage: sitemapPageSize, ExcludingFields: []string{"sections"},
		})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		var stories []storyblok.Story[storyblok.Blok]
		if err := json.Unmarshal(list.Stories, &stories); err != nil {
			s.fail(w, r, err)
			return
		}
		for _, story := range stories {
			if !components.IsPage(story.Content.Component) || isReservedSlug(story.FullSlug) {
				continue
			}
			versions := versionsOf(story.FullSlug, publishedAlternates(story.Alternates, storyblok.Published))
			var links []sitemapLink
			for _, a := range s.alternates(versions) {
				links = append(links, sitemapLink{Rel: "alternate", Hreflang: a.Lang, Href: a.Href})
			}
			lastMod := ""
			if !story.PublishedAt.IsZero() {
				lastMod = story.PublishedAt.UTC().Format(time.RFC3339)
			}
			_, inFolder := folderLocale(story.FullSlug)
			for _, v := range versions {
				// Folder-level versions are listed with their own story.
				own := v.path == storyPath(story.FullSlug)
				if own || (!inFolder && v.fieldLevel) {
					urls = append(urls, sitemapURL{Loc: s.siteURL + v.path, LastMod: lastMod, Alternates: links})
				}
			}
		}
		listed += len(stories)
		if len(stories) == 0 || listed >= list.Total {
			break
		}
	}
	slices.SortFunc(urls, func(a, b sitemapURL) int { return strings.Compare(a.Loc, b.Loc) })

	out, err := xml.Marshal(sitemapURLSet{XHTML: "http://www.w3.org/1999/xhtml", URLs: urls})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	setPageCaching(w.Header(), "")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

func (s *Server) serveRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	setPageCaching(w.Header(), "")
	_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	if s.siteURL != "" {
		_, _ = w.Write([]byte("Sitemap: " + s.siteURL + "/sitemap.xml\n"))
	}
}
