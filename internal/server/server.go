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
	"strconv"
	"strings"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/storyblok"
)

const (
	homeSlug = "home"
	// maxPreviewBodyBytes bounds the story JSON the Visual Editor sends.
	maxPreviewBodyBytes = 10 << 20

	// Browsers revalidate published pages on every view; the ETag keeps that
	// cheap. Not all browsers limit stale-while-revalidate to subresources, so
	// it is only granted to the shared cache in front (RFC 9213).
	pageCacheControl    = "no-cache"
	pageCDNCacheControl = "max-age=10, stale-while-revalidate=86400, stale-if-error=86400"
)

type ContentSource interface {
	Story(ctx context.Context, slug string, opts storyblok.StoryOptions) (jsontext.Value, error)
	CacheVersion() (cv int64, confirmed bool)
}

type Server struct {
	content      ContentSource
	renderer     *components.Renderer
	assets       fs.FS
	previewToken string
	logger       *slog.Logger
	now          func() time.Time
	buildID      string
}

type Option func(*Server)

// WithBuildID enables ETags for published pages. The ID must change whenever
// the same content renders differently, e.g. on every deploy.
func WithBuildID(id string) Option {
	return func(s *Server) { s.buildID = id }
}

func New(content ContentSource, renderer *components.Renderer, assets fs.FS, previewToken string, logger *slog.Logger, opts ...Option) *Server {
	s := &Server{
		content:      content,
		renderer:     renderer,
		assets:       assets,
		previewToken: previewToken,
		logger:       logger,
		now:          time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
	preview := storyblok.IsValidPreview(r.URL.Query(), s.previewToken, s.now())
	opts := storyblok.StoryOptions{
		Version:          storyblok.Published,
		ResolveRelations: components.ResolveRelations,
	}
	if preview {
		opts.Version = storyblok.Draft
	}

	// Read before fetching: the story reflects at least this cv, so the ETag
	// can only understate it, which costs a render but never serves stale
	// content.
	cv, confirmed := s.content.CacheVersion()
	etag := s.pageETag(cv)
	if !preview && confirmed && etag != "" && etagMatches(r.Header.Get("If-None-Match"), etag) {
		setPageCaching(w.Header(), etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	var story storyblok.Story[components.AnyBlock]
	storyErr := s.fetch(r.Context(), slug, opts, &story)

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

	var buf bytes.Buffer
	if err := s.renderer.Page(&buf, page); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if preview {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		setPageCaching(w.Header(), etag)
	}
	_, _ = buf.WriteTo(w)
}

// pageETag identifies a published page by the code that renders it and the
// content version it reflects.
func (s *Server) pageETag(cv int64) string {
	if s.buildID == "" || cv == 0 {
		return ""
	}
	return `"` + s.buildID + "-" + strconv.FormatInt(cv, 10) + `"`
}

func setPageCaching(h http.Header, etag string) {
	h.Set("Cache-Control", pageCacheControl)
	h.Set("CDN-Cache-Control", pageCDNCacheControl)
	if etag != "" {
		h.Set("ETag", etag)
	}
}

// etagMatches compares weakly, as If-None-Match requires; compressing proxies
// weaken the ETags they pass on.
func etagMatches(ifNoneMatch, etag string) bool {
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		if strings.TrimPrefix(strings.TrimSpace(candidate), "W/") == etag {
			return true
		}
	}
	return false
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
	status := http.StatusInternalServerError
	if errors.Is(err, storyblok.ErrRateLimited) {
		status = http.StatusServiceUnavailable
	}
	http.Error(w, http.StatusText(status), status)
}
