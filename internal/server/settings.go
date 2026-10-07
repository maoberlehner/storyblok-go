package server

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

const settingsSlug = "settings"

// reservedSlugs hold site data, not pages. They only render in previews.
var reservedSlugs = []string{settingsSlug, notFoundSlug}

func isReservedSlug(slug string) bool { return slices.Contains(reservedSlugs, slug) }

// settings returns the site settings in loc. If Storyblok fails, pages keep
// the last published settings; a missing settings story means no chrome.
func (s *Server) settings(ctx context.Context, version storyblok.Version, loc locale.Locale) *components.SiteSettings {
	last := s.lastSettings[loc.Code]
	story, err := s.fetch(ctx, settingsSlug, version, loc.StoryLanguage(), loc)
	if errors.Is(err, storyblok.ErrNotFound) {
		return nil
	}
	if err == nil {
		if settings, ok := story.Content.Block.(*components.SiteSettings); ok {
			if version != storyblok.Draft {
				last.Store(settings)
			}
			return settings
		}
		err = fmt.Errorf("content type %s", story.Content.Block.Meta().Component)
	}
	s.logger.WarnContext(ctx, "site settings unavailable", "locale", loc.Code, "err", err)
	return last.Load()
}

func (s *Server) chrome(ctx context.Context, version storyblok.Version, loc locale.Locale, path string, versions []langVersion) *components.Chrome {
	settings := s.settings(ctx, version, loc)
	if settings == nil {
		return nil
	}
	return &components.Chrome{Settings: settings, HomeHref: loc.HomePath(), CurrentPath: path, Languages: languageLinks(loc, versions)}
}
