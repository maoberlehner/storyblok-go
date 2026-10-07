package server

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/storyblok"
)

const settingsSlug = "settings"

// reservedSlugs hold site data, not pages. They only render in previews.
var reservedSlugs = []string{settingsSlug, notFoundSlug}

func isReservedSlug(slug string) bool { return slices.Contains(reservedSlugs, slug) }

// settings returns the site settings. If Storyblok fails, pages keep the last
// published settings; a missing settings story means no chrome.
func (s *Server) settings(ctx context.Context, version storyblok.Version) *components.SiteSettings {
	raw, err := s.content.Story(ctx, settingsSlug, storyblok.StoryOptions{Version: version})
	if errors.Is(err, storyblok.ErrNotFound) {
		return nil
	}
	if err == nil {
		var story storyblok.Story[components.AnyBlock]
		if err = json.Unmarshal(raw, &story); err == nil {
			if settings, ok := story.Content.Block.(*components.SiteSettings); ok {
				if version != storyblok.Draft {
					s.lastSettings.Store(settings)
				}
				return settings
			}
			err = fmt.Errorf("content type %s", story.Content.Block.Meta().Component)
		}
	}
	s.logger.WarnContext(ctx, "site settings unavailable", "err", err)
	return s.lastSettings.Load()
}

// chrome returns the header and footer for a page at path.
func (s *Server) chrome(ctx context.Context, version storyblok.Version, path string) *components.Chrome {
	settings := s.settings(ctx, version)
	if settings == nil {
		return nil
	}
	return &components.Chrome{Settings: settings, HomeHref: "/", CurrentPath: path}
}
