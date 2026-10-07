package server

import (
	"context"
	"encoding/json/v2"
	"net/http"

	"storyblok-go-website/internal/storyblok"
)

// iconCacheControl allows a day of caching for icons, whose URLs browsers and
// platforms expect at fixed, unversioned paths.
const iconCacheControl = "public, max-age=86400"

// serveIcon serves an icon from the assets at a well-known root path.
func (s *Server) serveIcon(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", iconCacheControl)
		http.ServeFileFS(w, r, s.assets, "icons/"+name)
	}
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

func (s *Server) serveManifest(w http.ResponseWriter, r *http.Request) {
	name := s.siteName(r.Context())
	manifest := map[string]any{
		"name":             name,
		"short_name":       name,
		"start_url":        "/",
		"display":          "browser",
		"background_color": "#ffffff",
		"theme_color":      "#184db5",
		"icons": []manifestIcon{
			{Src: "/assets/icons/icon-192.png", Sizes: "192x192", Type: "image/png", Purpose: "any"},
			{Src: "/assets/icons/icon-512.png", Sizes: "512x512", Type: "image/png", Purpose: "any"},
			{Src: "/assets/icons/icon-maskable-512.png", Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", iconCacheControl)
	_ = json.MarshalWrite(w, manifest)
}

// siteName names the site in the web app manifest.
func (s *Server) siteName(ctx context.Context) string {
	if settings := s.settings(ctx, storyblok.Published); settings != nil && settings.SiteName != "" {
		return settings.SiteName
	}
	return "Website"
}
