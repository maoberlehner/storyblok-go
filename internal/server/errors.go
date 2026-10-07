package server

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"net/http"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/storyblok"
)

const notFoundSlug = "error-404"

type errorText struct{ heading, text string }

var errorTexts = map[int]errorText{
	http.StatusNotFound:            {"Page not found", "The page you are looking for doesn't exist or has moved."},
	http.StatusInternalServerError: {"Something went wrong", "Please try again in a moment."},
	http.StatusServiceUnavailable:  {"Something went wrong", "Please try again in a moment."},
}

const homeLabel = "Go to the home page"

// plainErrors reports whether the client expects a fragment or data, not a
// document: htmx requests and Visual Editor preview renders.
func plainErrors(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" || r.Method == http.MethodPut
}

// notFound renders the "error-404" story, or a built-in page if it is missing.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request, version storyblok.Version) {
	if plainErrors(r) {
		http.NotFound(w, r)
		return
	}
	var page components.Page
	raw, err := s.content.Story(r.Context(), notFoundSlug, storyblok.StoryOptions{Version: version, ResolveRelations: components.ResolveRelations})
	var story storyblok.Story[components.AnyBlock]
	if err == nil {
		err = json.Unmarshal(raw, &story)
	}
	if err == nil && components.IsPage(story.Content.Block.Meta().Component) {
		page = components.NewPage(story)
	} else {
		if err != nil && !errors.Is(err, storyblok.ErrNotFound) {
			s.logger.WarnContext(r.Context(), "404 page unavailable", "err", err)
		}
		text := errorTexts[http.StatusNotFound]
		page = components.NewPage(storyblok.Story[components.AnyBlock]{Content: components.AnyBlock{
			Block: components.NewErrorContent(text.heading, text.text, "/", homeLabel),
		}})
	}
	page.NoIndex = true
	page.Chrome = s.chrome(r.Context(), version, r.URL.Path)
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
	text := errorTexts[status]
	page := components.NewPage(storyblok.Story[components.AnyBlock]{Content: components.AnyBlock{
		Block: components.NewErrorContent(text.heading, text.text, "/", homeLabel),
	}})
	page.NoIndex = true
	if settings := s.lastSettings.Load(); settings != nil {
		page.Chrome = &components.Chrome{Settings: settings, HomeHref: "/", CurrentPath: r.URL.Path}
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
