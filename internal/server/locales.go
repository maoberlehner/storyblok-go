package server

import (
	"cmp"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strings"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

// requestLocale is the URL's locale, or the Visual Editor's language in
// previews, which shows field-level translations at the default path.
func requestLocale(r *http.Request, preview bool) (locale.Locale, string) {
	loc, rest := locale.FromPath(r.URL.Path)
	if preview {
		if l, ok := locale.ByCode(r.URL.Query().Get("_storyblok_lang")); ok {
			loc = l
		}
	}
	return loc, rest
}

// resolution is the story a URL shows.
type resolution struct {
	story  storyblok.Story[components.AnyBlock]
	locale locale.Locale
	// redirect is the URL of the story's folder-level translation, which
	// replaces its field-level one.
	redirect string
	// defaultSlug is the default-language slug of a field-level translation.
	// The API's full_slug carries the language prefix (unless the space turns
	// it off) and default_full_slug is only set with translatable slugs.
	defaultSlug string
}

// path is the URL of the resolved story.
func (res resolution) path() string {
	if res.defaultSlug == "" {
		return storyPath(res.story.FullSlug)
	}
	if res.locale.IsDefault() {
		return storyPath(res.defaultSlug)
	}
	return storyPath(res.locale.Code + "/" + res.defaultSlug)
}

// resolve finds the story for rest in loc: a folder-level translation under
// "<code>/" first, then the field-level translation of the default story.
func (s *Server) resolve(ctx context.Context, loc locale.Locale, rest string, version storyblok.Version) (resolution, error) {
	slug := cmp.Or(rest, homeSlug)
	if isReservedSlug(slug) && version != storyblok.Draft {
		return resolution{}, storyblok.ErrNotFound
	}
	if _, inFolder := folderLocale(slug); inFolder {
		return resolution{}, storyblok.ErrNotFound
	}
	if !loc.IsDefault() {
		story, err := s.fetch(ctx, loc.Code+"/"+slug, version, "", loc)
		if err == nil {
			return resolution{story: story, locale: loc}, nil
		}
		if !errors.Is(err, storyblok.ErrNotFound) {
			return resolution{}, err
		}
	}
	story, err := s.fetch(ctx, slug, version, loc.StoryLanguage(), loc)
	if err != nil {
		return resolution{}, err
	}
	res := resolution{story: story, locale: loc, defaultSlug: slug}
	if !loc.IsDefault() {
		for _, alt := range publishedAlternates(story.Alternates, version) {
			if l, ok := folderLocale(alt.FullSlug); ok && l.Code == loc.Code {
				res.redirect = storyPath(alt.FullSlug)
			}
		}
	}
	return res, nil
}

// fetch loads and decodes a story, pointing its story links to loc's pages.
func (s *Server) fetch(ctx context.Context, slug string, version storyblok.Version, language string, loc locale.Locale) (storyblok.Story[components.AnyBlock], error) {
	var story storyblok.Story[components.AnyBlock]
	raw, err := s.content.Story(ctx, slug, storyblok.StoryOptions{Version: version, Language: language, ResolveRelations: components.ResolveRelations})
	if err != nil {
		return story, err
	}
	if raw, err = localizeLinks(raw, loc); err != nil {
		return story, err
	}
	return story, json.Unmarshal(raw, &story)
}

// localizeLinks points story links in content shown in a non-default locale
// to that locale's pages. The API prefixes links in field-level translations
// with the language, including links to folder-level translations, which
// already carry it.
func localizeLinks(raw jsontext.Value, loc locale.Locale) (jsontext.Value, error) {
	if loc.IsDefault() {
		return raw, nil
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	walkJSON(tree, func(node map[string]any) {
		if node["linktype"] != "story" {
			return
		}
		if slug, ok := node["cached_url"].(string); ok {
			node["cached_url"] = localizedSlug(slug, loc)
		}
	})
	return json.Marshal(tree)
}

func localizedSlug(slug string, loc locale.Locale) string {
	prefix := loc.Code + "/"
	slug = strings.Trim(slug, "/")
	for strings.HasPrefix(slug, prefix+prefix) {
		slug = strings.TrimPrefix(slug, prefix)
	}
	slug = strings.TrimPrefix(slug, prefix)
	if slug == "" || slug == homeSlug || slug == loc.Code {
		return loc.Code
	}
	return prefix + slug
}

func walkJSON(node any, visit func(map[string]any)) {
	switch n := node.(type) {
	case map[string]any:
		visit(n)
		for _, child := range n {
			walkJSON(child, visit)
		}
	case []any:
		for _, child := range n {
			walkJSON(child, visit)
		}
	}
}

// folderLocale reports the non-default locale whose folder holds slug.
func folderLocale(slug string) (locale.Locale, bool) {
	for _, l := range locale.All {
		if !l.IsDefault() && (slug == l.Code || strings.HasPrefix(slug, l.Code+"/")) {
			return l, true
		}
	}
	return locale.Locale{}, false
}

func publishedAlternates(alternates []storyblok.Alternate, version storyblok.Version) []storyblok.Alternate {
	var result []storyblok.Alternate
	for _, alt := range alternates {
		if alt.Published || version == storyblok.Draft {
			result = append(result, alt)
		}
	}
	return result
}

// langVersion is a page in one language.
type langVersion struct {
	locale locale.Locale
	path   string
	// fieldLevel versions have no story of their own.
	fieldLevel bool
}

// versionsOf lists the language versions of the story at fullSlug (a default
// language slug) given its alternates. The default version is the story
// outside locale folders; a locale's version is its folder-level alternate,
// else the field-level translation of the default story.
func versionsOf(fullSlug string, alternates []storyblok.Alternate) []langVersion {
	slugs := []string{fullSlug}
	for _, alt := range alternates {
		slugs = append(slugs, alt.FullSlug)
	}
	base := ""
	folders := map[string]string{}
	for _, slug := range slugs {
		if l, ok := folderLocale(slug); ok {
			if _, seen := folders[l.Code]; !seen {
				folders[l.Code] = slug
			}
		} else if base == "" {
			base = slug
		}
	}
	var versions []langVersion
	for _, l := range locale.All {
		switch {
		case l.IsDefault() && base != "":
			versions = append(versions, langVersion{locale: l, path: storyPath(base)})
		case l.IsDefault():
		case folders[l.Code] != "":
			versions = append(versions, langVersion{locale: l, path: storyPath(folders[l.Code])})
		case base != "":
			versions = append(versions, langVersion{locale: l, path: storyPath(l.Code + "/" + base), fieldLevel: true})
		}
	}
	return versions
}

// versions lists the language versions of a resolved story.
func (res resolution) versions(version storyblok.Version) []langVersion {
	slug := cmp.Or(res.defaultSlug, res.story.FullSlug)
	return versionsOf(slug, publishedAlternates(res.story.Alternates, version))
}

// languageLinks are the language switcher's links: each locale's version of
// the page, or its home page.
func languageLinks(current locale.Locale, versions []langVersion) []components.LanguageLink {
	var links []components.LanguageLink
	for _, l := range locale.All {
		link := components.LanguageLink{Lang: l.Code, Label: l.Name, Href: l.HomePath(), Current: l.Code == current.Code}
		for _, v := range versions {
			if v.locale.Code == l.Code {
				link.Href = v.path
			}
		}
		links = append(links, link)
	}
	return links
}

// alternates are the hreflang links of a published page with several versions.
func (s *Server) alternates(versions []langVersion) []components.Alternate {
	if s.siteURL == "" || len(versions) < 2 {
		return nil
	}
	var result []components.Alternate
	for _, v := range versions {
		result = append(result, components.Alternate{Lang: v.locale.Code, Href: s.siteURL + v.path, OG: v.locale.OG})
		if v.locale.IsDefault() {
			result = append(result, components.Alternate{Lang: "x-default", Href: s.siteURL + v.path})
		}
	}
	return result
}
