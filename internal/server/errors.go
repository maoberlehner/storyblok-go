package server

import (
	"bytes"
	"errors"
	"net/http"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

const notFoundSlug = "error-404"

// plainErrors reports whether the client expects a fragment or data, not a
// document: htmx requests and Visual Editor preview renders.
func plainErrors(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" || r.Method == http.MethodPut
}

// errorPage is a built-in page for errors, in loc.
func errorPage(loc locale.Locale, heading, text string) components.Page {
	page := components.NewPage(storyblok.Story[components.AnyBlock]{Content: components.AnyBlock{
		Block: components.NewErrorContent(heading, text, loc.HomePath(), loc.T("error.home")),
	}})
	page.Locale = loc
	page.NoIndex = true
	return page
}

// notFound renders the "error-404" story in loc, or a built-in page if it is
// missing.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request, version storyblok.Version, loc locale.Locale) {
	if plainErrors(r) {
		http.NotFound(w, r)
		return
	}
	var page components.Page
	story, err := s.fetch(r.Context(), notFoundSlug, version, loc.StoryLanguage(), loc)
	if err == nil && components.IsPage(story.Content.Block.Meta().Component) {
		page = components.NewPage(story)
		page.Locale = loc
		page.NoIndex = true
	} else {
		if err != nil && !errors.Is(err, storyblok.ErrNotFound) {
			s.logger.WarnContext(r.Context(), "404 page unavailable", "err", err)
		}
		page = errorPage(loc, loc.T("error.not_found"), loc.T("error.not_found_text"))
	}
	page.Chrome = s.chrome(r.Context(), version, loc, r.URL.Path, nil)
	s.writePage(w, r, http.StatusNotFound, page)
}

// fail logs err and answers with an error page. It never calls Storyblok:
// the header and footer come from the last settings that loaded.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	status := http.StatusInternalServerError
	if errors.Is(err, storyblok.ErrRateLimited) {
		status = http.StatusServiceUnavailable
	}
	if plainErrors(r) {
		http.Error(w, http.StatusText(status), status)
		return
	}
	loc, _ := locale.FromPath(r.URL.Path)
	page := errorPage(loc, loc.T("error.server"), loc.T("error.server_text"))
	if settings := s.lastSettings[loc.Code].Load(); settings != nil {
		page.Chrome = &components.Chrome{Settings: settings, HomeHref: loc.HomePath(), CurrentPath: r.URL.Path,
			Languages: languageLinks(loc, nil)}
	}
	var buf bytes.Buffer
	if renderErr := s.renderer.Page(&buf, page); renderErr != nil {
		s.logger.ErrorContext(r.Context(), "rendering error page failed", "err", renderErr)
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
