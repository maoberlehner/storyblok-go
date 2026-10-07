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
	"net/url"
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
	maxFormBodyBytes    = 64 << 10
	vendorAssetsPrefix  = "/assets/vendor/"

	// Browsers revalidate published pages on every view; the ETag keeps that
	// cheap. Not all browsers limit stale-while-revalidate to subresources, so
	// it is only granted to the shared cache in front (RFC 9213).
	pageCacheControl    = "no-cache"
	pageCDNCacheControl = "max-age=10, stale-while-revalidate=86400, stale-if-error=86400"
)

type ContentSource interface {
	components.Content
	Story(ctx context.Context, slug string, opts storyblok.StoryOptions) (jsontext.Value, error)
	CacheVersion() (cv int64, confirmed bool)
}

type Server struct {
	content      ContentSource
	renderer     *components.Renderer
	assets       fs.FS
	inbox        components.Inbox
	previewToken string
	logger       *slog.Logger
	now          func() time.Time
	buildID      string
	siteURL      string
	recordVital  func(name string, value float64)
}

type Option func(*Server)

// WithBuildID enables ETags for published pages. The ID must change whenever
// the same content renders differently, e.g. on every deploy.
func WithBuildID(id string) Option {
	return func(s *Server) { s.buildID = id }
}

// WithSiteURL sets the public origin, such as https://www.example.com, for
// canonical URLs and the sitemap.
func WithSiteURL(origin string) Option {
	return func(s *Server) { s.siteURL = strings.TrimSuffix(origin, "/") }
}

func New(content ContentSource, renderer *components.Renderer, assets fs.FS, inbox components.Inbox, previewToken string, logger *slog.Logger, opts ...Option) *Server {
	s := &Server{
		content:      content,
		renderer:     renderer,
		assets:       assets,
		inbox:        inbox,
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
	mux.HandleFunc("GET /assets/app.js", s.serveBundle("text/javascript; charset=utf-8", s.renderer.Script))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", s.serveAsset(http.FileServerFS(s.assets))))
	mux.HandleFunc("GET /healthz", s.serveHealthz)
	mux.HandleFunc("POST /vitals", s.receiveVital)
	mux.HandleFunc("GET /sitemap.xml", s.serveSitemap)
	mux.HandleFunc("GET /robots.txt", s.serveRobots)
	mux.HandleFunc("GET /{slug...}", s.showStory)
	mux.HandleFunc("POST /{slug...}", s.submitForm)
	mux.HandleFunc("PUT /{slug...}", s.previewStory)
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux), s.siteURL)
}

func (s *Server) showStory(w http.ResponseWriter, r *http.Request) {
	preview := storyblok.IsValidPreview(r.URL.Query(), s.previewToken, s.now())
	version := storyblok.Published
	if preview {
		version = storyblok.Draft
	}
	req := s.componentRequest(r, version)
	w.Header().Add("Vary", "HX-Request")

	// Read before fetching: the story reflects at least this cv, so the ETag
	// can only understate it, which costs a render but never serves stale
	// content.
	cv, confirmed := s.content.CacheVersion()
	etag := s.pageETag(cv, req.Enhanced)
	if !preview && confirmed && etag != "" && etagMatches(r.Header.Get("If-None-Match"), etag) {
		setPageCaching(w.Header(), etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	story, ok := s.story(w, r, version)
	if !ok {
		return
	}
	if preview {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		setPageCaching(w.Header(), etag)
	}

	// Enhanced requests addressed to a section only render its fragment.
	if target, ok := components.FindSection(story.Content.Block, req.Query.Get(components.TargetParam)); ok && req.Enhanced {
		if err := components.Load(r.Context(), s.content, target, req); err != nil {
			s.fail(w, r, err)
			return
		}
		// Reloading the page then shows the same state without JavaScript.
		w.Header().Set("HX-Replace-Url", replaceURL(r, req, target))
		s.writeFragment(w, r, http.StatusOK, target)
		return
	}

	if err := components.LoadSections(r.Context(), s.content, story.Content.Block, req); err != nil {
		s.fail(w, r, err)
		return
	}
	page := components.NewPage(story)
	page.Preview = preview
	page.Canonical = s.canonicalURL(story.FullSlug)
	s.writePage(w, r, http.StatusOK, page)
}

// submitForm handles forms of the page's sections. Without JavaScript, valid
// submissions redirect to the page, which confirms them; invalid ones render
// the page with errors. Enhanced requests get the form's fragment instead.
// The confirmation and the error summary take focus with autofocus, which
// browsers skip for URLs with a fragment.
func (s *Server) submitForm(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	story, ok := s.story(w, r, storyblok.Published)
	if !ok {
		return
	}
	section, _ := components.FindSection(story.Content.Block, r.PostForm.Get(components.TargetParam))
	form, ok := section.(components.FormHandler)
	if !ok {
		http.Error(w, "unknown form", http.StatusBadRequest)
		return
	}

	req := s.componentRequest(r, storyblok.Published)
	load := func() error { return components.Load(r.Context(), s.content, form, req) }
	if !req.Enhanced {
		load = func() error { return components.LoadSections(r.Context(), s.content, story.Content.Block, req) }
	}
	if err := load(); err != nil {
		s.fail(w, r, err)
		return
	}
	valid, err := form.Submit(r.Context(), s.inbox, r.PostForm)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	status := http.StatusOK
	if !valid {
		status = http.StatusUnprocessableEntity
	}
	switch {
	case req.Enhanced:
		s.writeFragment(w, r, status, form)
	case valid:
		query := req.StateQuery()
		query.Set(components.SentParam, components.ShortID(form))
		http.Redirect(w, r, req.Path+"?"+query.Encode(), http.StatusSeeOther)
	default:
		page := components.NewPage(story)
		page.Title = "Error: " + page.Title
		page.Canonical = s.canonicalURL(story.FullSlug)
		s.writePage(w, r, status, page)
	}
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
	req := s.componentRequest(r, storyblok.Draft)
	if err := components.LoadSections(r.Context(), s.content, story.Content.Block, req); err != nil {
		s.fail(w, r, err)
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

// story fetches the story at the request path. It responds with an error and
// returns false if that fails.
func (s *Server) story(w http.ResponseWriter, r *http.Request, version storyblok.Version) (storyblok.Story[components.AnyBlock], bool) {
	slug := cmp.Or(strings.Trim(r.PathValue("slug"), "/"), homeSlug)
	var story storyblok.Story[components.AnyBlock]
	raw, err := s.content.Story(r.Context(), slug, storyblok.StoryOptions{
		Version:          version,
		ResolveRelations: components.ResolveRelations,
	})
	if err == nil {
		err = json.Unmarshal(raw, &story)
	}
	switch {
	case errors.Is(err, storyblok.ErrNotFound):
		http.NotFound(w, r)
		return story, false
	case err != nil:
		s.fail(w, r, err)
		return story, false
	}
	return story, true
}

// canonicalURL is the story's URL without query parameters, which only hold
// view state such as loaded pages or a form confirmation.
func (s *Server) canonicalURL(fullSlug string) string {
	if s.siteURL == "" {
		return ""
	}
	return s.siteURL + storyPath(fullSlug)
}

func storyPath(fullSlug string) string {
	slug := strings.Trim(fullSlug, "/")
	if slug == homeSlug {
		slug = ""
	}
	return "/" + slug
}

func (s *Server) componentRequest(r *http.Request, version storyblok.Version) components.Request {
	return components.Request{
		Path:     r.URL.Path,
		Query:    r.URL.Query(),
		Version:  version,
		Enhanced: r.Header.Get("HX-Request") == "true",
	}
}

func (s *Server) writePage(w http.ResponseWriter, r *http.Request, status int, page components.Page) {
	var buf bytes.Buffer
	if err := s.renderer.Page(&buf, page); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) writeFragment(w http.ResponseWriter, r *http.Request, status int, block components.Block) {
	var buf bytes.Buffer
	if err := s.renderer.Fragment(&buf, block); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) serveBundle(contentType string, content func() []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		if r.URL.Query().Has("v") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		_, _ = w.Write(content())
	}
}

// serveAsset marks versioned vendor files as cacheable forever.
func (s *Server) serveAsset(files http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix("/assets/"+r.URL.Path, vendorAssetsPrefix) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	status := http.StatusInternalServerError
	if errors.Is(err, storyblok.ErrRateLimited) {
		status = http.StatusServiceUnavailable
	}
	http.Error(w, http.StatusText(status), status)
}

// replaceURL returns the browser URL after an enhanced request to target: the
// current URL with target's state from the request. Other blocks' values in
// the request may predate earlier enhanced requests, so they are ignored.
func replaceURL(r *http.Request, req components.Request, target components.Block) string {
	state := url.Values{}
	if current, err := url.Parse(r.Header.Get("HX-Current-URL")); err == nil {
		state = current.Query()
		// A confirmation shown before should not reappear on reload.
		state.Del(components.SentParam)
	}
	for _, param := range components.StateParams(target) {
		state.Set(param, req.Query.Get(param))
	}
	if len(state) == 0 {
		return req.Path
	}
	return req.Path + "?" + state.Encode()
}

// pageETag identifies a published page by the code that renders it, the
// content version it reflects, and whether it is the htmx fragment.
func (s *Server) pageETag(cv int64, fragment bool) string {
	if s.buildID == "" || cv == 0 {
		return ""
	}
	etag := s.buildID + "-" + strconv.FormatInt(cv, 10)
	if fragment {
		etag += "-fragment"
	}
	return `"` + etag + `"`
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

// LogInbox logs form submissions without their values, standing in for a
// delivery to a CRM or mailbox.
type LogInbox struct {
	Logger *slog.Logger
}

func (i LogInbox) Deliver(ctx context.Context, s components.Submission) error {
	attrs := []any{"form", s.Form}
	for _, f := range s.Fields {
		attrs = append(attrs, f.Name+"_chars", len([]rune(f.Value)))
	}
	i.Logger.InfoContext(ctx, "form submitted", attrs...)
	return nil
}
