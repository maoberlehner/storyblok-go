// Package server serves Storyblok stories as HTML pages and renders live
// previews for the Visual Editor.
package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/locale"
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
	lastSettings map[string]*atomic.Pointer[components.SiteSettings]
	formSecret   []byte
	guard        components.FormGuard
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

// WithFormSecret signs form tokens. All instances behind one site need the
// same secret.
func WithFormSecret(secret []byte) Option {
	return func(s *Server) { s.formSecret = secret }
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
	s.lastSettings = map[string]*atomic.Pointer[components.SiteSettings]{}
	for _, l := range locale.All {
		s.lastSettings[l.Code] = new(atomic.Pointer[components.SiteSettings])
	}
	if len(s.formSecret) == 0 {
		// Tokens from other processes fail with a random secret; production
		// sets FORM_SECRET.
		s.formSecret = make([]byte, 32)
		_, _ = rand.Read(s.formSecret)
	}
	s.guard = components.NewFormGuard(s.formSecret, s.now)
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /assets/app.js", s.serveBundle("text/javascript; charset=utf-8", s.renderer.Script))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", s.serveAsset(http.FileServerFS(s.assets))))
	mux.HandleFunc("GET /healthz", s.serveHealthz)
	mux.HandleFunc("GET /favicon.ico", s.serveIcon("favicon.ico"))
	mux.HandleFunc("GET /apple-touch-icon.png", s.serveIcon("apple-touch-icon.png"))
	mux.HandleFunc("GET /manifest.webmanifest", s.serveManifest)
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
	loc, _ := requestLocale(r, preview)
	req := s.componentRequest(r, version, loc)
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

	res, ok := s.story(w, r, version)
	if !ok {
		return
	}
	if preview {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		setPageCaching(w.Header(), etag)
	}

	// Enhanced requests addressed to a section only render its fragment.
	if target, ok := components.FindSection(res.story.Content.Block, req.Query.Get(components.TargetParam)); ok && req.Enhanced {
		if err := components.Load(r.Context(), s.content, target, req); err != nil {
			s.fail(w, r, err)
			return
		}
		// Reloading the page then shows the same state without JavaScript.
		w.Header().Set("HX-Replace-Url", replaceURL(r, req, target))
		s.writeFragment(w, r, http.StatusOK, target, res.locale)
		return
	}

	if err := components.LoadSections(r.Context(), s.content, res.story.Content.Block, req); err != nil {
		s.fail(w, r, err)
		return
	}
	page := components.NewPage(res.story)
	page.Preview = preview
	page.Locale = res.locale
	page.Canonical = s.canonicalURL(res.story.FullSlug)
	versions := res.versions(version)
	if !preview {
		page.Alternates = s.alternates(versions)
	}
	page.Chrome = s.chrome(r.Context(), version, res.locale, r.URL.Path, versions)
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
	res, ok := s.story(w, r, storyblok.Published)
	if !ok {
		return
	}
	section, _ := components.FindSection(res.story.Content.Block, r.PostForm.Get(components.TargetParam))
	form, ok := section.(components.FormHandler)
	if !ok {
		http.Error(w, "unknown form", http.StatusBadRequest)
		return
	}

	req := s.componentRequest(r, storyblok.Published, res.locale)
	load := func() error { return components.Load(r.Context(), s.content, form, req) }
	if !req.Enhanced {
		load = func() error { return components.LoadSections(r.Context(), s.content, res.story.Content.Block, req) }
	}
	if err := load(); err != nil {
		s.fail(w, r, err)
		return
	}
	valid := false
	switch s.guard.Check(r.PostForm) {
	case components.Bot:
		s.logger.InfoContext(r.Context(), "form spam dropped", "form", form.Meta().Component, "path", r.URL.Path)
		form.Confirm()
		valid = true
	case components.TooFast:
		form.Reject(r.PostForm, res.locale.T("form.too_fast"))
	default:
		var err error
		if valid, err = form.Submit(r.Context(), s.inbox, r.PostForm); err != nil {
			s.fail(w, r, err)
			return
		}
	}

	status := http.StatusOK
	if !valid {
		status = http.StatusUnprocessableEntity
	}
	switch {
	case req.Enhanced:
		s.writeFragment(w, r, status, form, res.locale)
	case valid:
		query := req.StateQuery()
		query.Set(components.SentParam, components.ShortID(form))
		http.Redirect(w, r, req.Path+"?"+query.Encode(), http.StatusSeeOther)
	default:
		page := components.NewPage(res.story)
		page.Title = res.locale.T("error.title_prefix", page.Title)
		page.Locale = res.locale
		page.Canonical = s.canonicalURL(res.story.FullSlug)
		versions := res.versions(storyblok.Published)
		page.Alternates = s.alternates(versions)
		page.Chrome = s.chrome(r.Context(), storyblok.Published, res.locale, r.URL.Path, versions)
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
	loc, _ := requestLocale(r, true)
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPreviewBodyBytes))
	if err == nil {
		raw, err = localizeLinks(raw, loc)
	}
	var story storyblok.Story[components.AnyBlock]
	if err == nil {
		err = json.Unmarshal(raw, &story)
	}
	if err != nil {
		http.Error(w, "invalid story: "+err.Error(), http.StatusBadRequest)
		return
	}
	req := s.componentRequest(r, storyblok.Draft, loc)
	if err := components.LoadSections(r.Context(), s.content, story.Content.Block, req); err != nil {
		s.fail(w, r, err)
		return
	}

	var buf bytes.Buffer
	if err := s.renderer.Block(&buf, story.Content.Block, loc); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = buf.WriteTo(w)
}

// story resolves the story at the request path. It responds and returns
// false if there is none, it redirects, or fetching fails.
func (s *Server) story(w http.ResponseWriter, r *http.Request, version storyblok.Version) (resolution, bool) {
	loc, rest := requestLocale(r, version == storyblok.Draft)
	res, err := s.resolve(r.Context(), loc, rest, version)
	switch {
	case errors.Is(err, storyblok.ErrNotFound):
		s.notFound(w, r, version, loc)
		return res, false
	case err != nil:
		s.fail(w, r, err)
		return res, false
	case res.redirect != "" && r.Method == http.MethodGet:
		http.Redirect(w, r, res.redirect, http.StatusMovedPermanently)
		return res, false
	case res.redirect != "":
		s.notFound(w, r, version, loc)
		return res, false
	}
	return res, true
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
		return "/"
	}
	if l, ok := folderLocale(slug); ok && (slug == l.Code || slug == l.Code+"/"+homeSlug) {
		return "/" + l.Code
	}
	return "/" + slug
}

func (s *Server) componentRequest(r *http.Request, version storyblok.Version, loc locale.Locale) components.Request {
	return components.Request{
		Path:      r.URL.Path,
		Query:     r.URL.Query(),
		Version:   version,
		Enhanced:  r.Header.Get("HX-Request") == "true",
		FormToken: s.guard.Token(),
		Locale:    loc,
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

func (s *Server) writeFragment(w http.ResponseWriter, r *http.Request, status int, block components.Block, loc locale.Locale) {
	var buf bytes.Buffer
	if err := s.renderer.Fragment(&buf, block, loc); err != nil {
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
		if strings.HasPrefix(r.URL.Path, "icons/") {
			w.Header().Set("Cache-Control", iconCacheControl)
		}
		files.ServeHTTP(w, r)
	})
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
	for _, f := range s.Attribution {
		attrs = append(attrs, "attribution_"+f.Name, f.Value)
	}
	i.Logger.InfoContext(ctx, "form submitted", attrs...)
	return nil
}
