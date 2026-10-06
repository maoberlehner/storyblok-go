// Package server serves Storyblok stories as HTML pages and renders live
// previews for the Visual Editor.
package server

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/storyblok"
)

const (
	homeSlug   = "home"
	configSlug = "config"
	// maxPreviewBodyBytes bounds the story JSON the Visual Editor sends.
	maxPreviewBodyBytes = 10 << 20
)

type ContentSource interface {
	Story(ctx context.Context, slug string, opts storyblok.StoryOptions) (jsontext.Value, error)
}

type Server struct {
	content      ContentSource
	renderer     *components.Renderer
	assets       fs.FS
	previewToken string
	logger       *slog.Logger
	now          func() time.Time
}

func New(content ContentSource, renderer *components.Renderer, assets fs.FS, previewToken string, logger *slog.Logger) *Server {
	return &Server{
		content:      content,
		renderer:     renderer,
		assets:       assets,
		previewToken: previewToken,
		logger:       logger,
		now:          time.Now,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /assets/app.css", s.serveStylesheet)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(s.assets)))
	mux.HandleFunc("GET /{slug...}", s.showStory)
	mux.HandleFunc("PUT /{slug...}", s.previewStory)
	return mux
}

func (s *Server) showStory(w http.ResponseWriter, r *http.Request) {
	slug := cmp.Or(strings.Trim(r.PathValue("slug"), "/"), homeSlug)
	if slug == configSlug {
		http.NotFound(w, r)
		return
	}
	preview := storyblok.IsValidPreview(r.URL.Query(), s.previewToken, s.now())
	opts := storyblok.StoryOptions{
		Version:          storyblok.Published,
		ResolveRelations: components.ResolveRelations,
	}
	if preview {
		opts.Version = storyblok.Draft
	}

	var (
		story               storyblok.Story[components.AnyBlock]
		config              storyblok.Story[components.Configuration]
		storyErr, configErr error
	)
	var wg sync.WaitGroup
	wg.Go(func() { storyErr = s.fetch(r.Context(), slug, opts, &story) })
	wg.Go(func() { configErr = s.fetch(r.Context(), configSlug, opts, &config) })
	wg.Wait()

	switch {
	case errors.Is(storyErr, storyblok.ErrNotFound):
		http.NotFound(w, r)
		return
	case storyErr != nil:
		s.fail(w, r, storyErr)
		return
	}

	page := components.NewPage(story)
	page.Preview = preview
	if configErr == nil {
		page.Config = &config.Content
	} else {
		s.logger.WarnContext(r.Context(), "rendering without site configuration", "err", configErr)
	}

	var buf bytes.Buffer
	if err := s.renderer.Page(&buf, page); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if preview {
		w.Header().Set("Cache-Control", "no-store")
	}
	_, _ = buf.WriteTo(w)
}

// previewStory renders a story sent by the Visual Editor bridge and returns
// the markup of the page's main content for the live preview to morph in.
func (s *Server) previewStory(w http.ResponseWriter, r *http.Request) {
	if !storyblok.IsValidPreview(r.URL.Query(), s.previewToken, s.now()) {
		http.Error(w, "invalid or expired preview token", http.StatusForbidden)
		return
	}
	var story storyblok.Story[components.AnyBlock]
	if err := json.UnmarshalRead(http.MaxBytesReader(w, r.Body, maxPreviewBodyBytes), &story); err != nil {
		http.Error(w, "invalid story: "+err.Error(), http.StatusBadRequest)
		return
	}

	var buf bytes.Buffer
	if err := s.renderer.Block(&buf, story.Content.Block); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = buf.WriteTo(w)
}

func (s *Server) serveStylesheet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	if r.URL.Query().Has("v") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	_, _ = w.Write(s.renderer.Stylesheet())
}

func (s *Server) fetch(ctx context.Context, slug string, opts storyblok.StoryOptions, out any) error {
	raw, err := s.content.Story(ctx, slug, opts)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
