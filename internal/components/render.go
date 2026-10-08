package components

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

var (
	//go:embed *.html
	templateFS embed.FS
	//go:embed *.css *.js
	assetFS embed.FS
)

// ResolveRelations lists the relation fields components expect as resolved
// stories, both from the Content Delivery API and the Visual Editor bridge.
var ResolveRelations []string

type Metadata struct {
	Title, Description       string
	SEOTitle, SEODescription string
	OGTitle, OGDescription   string
	OGImage                  storyblok.Asset
	// Type is the Open Graph type; empty means "website".
	Type string
}

// Share images are cropped to the size link previews display.
const ShareImageWidth, ShareImageHeight = 1200, 630

type ShareImage struct {
	URL, Alt string
}

func (p Page) DocumentTitle() string    { return cmp.Or(p.SEOTitle, p.Title) }
func (p Page) MetaDescription() string  { return cmp.Or(p.SEODescription, p.Description) }
func (p Page) ShareTitle() string       { return cmp.Or(p.OGTitle, p.DocumentTitle()) }
func (p Page) ShareDescription() string { return cmp.Or(p.OGDescription, p.MetaDescription()) }
func (p Page) OGType() string           { return cmp.Or(p.Type, "website") }

func (p Page) settings() *SiteSettings {
	if p.Chrome == nil {
		return nil
	}
	return p.Chrome.Settings
}

func (p Page) SiteName() string {
	if s := p.settings(); s != nil {
		return s.SiteName
	}
	return ""
}

// ShareImage is the page's image for link previews, else the site's default.
// Link previews don't support SVG.
func (p Page) ShareImage() *ShareImage {
	image := p.OGImage
	if image.IsZero() {
		if s := p.settings(); s != nil {
			image = s.DefaultOGImage
		}
	}
	if image.IsZero() || image.IsSVG() {
		return nil
	}
	return &ShareImage{
		URL: image.Image(storyblok.ImageOptions{Width: ShareImageWidth, Height: ShareImageHeight}),
		Alt: image.Alt,
	}
}

// Page is everything the layout needs to render a full document.
type Page struct {
	Metadata
	Content Block
	// Preview loads the Visual Editor bridge and live preview script, and
	// keeps the page out of search indexes.
	Preview bool
	// Canonical is the absolute URL search engines should index the page as.
	Canonical string
	// Chrome renders the site header and footer; nil renders the page
	// without them.
	Chrome *Chrome
	// NoIndex keeps the page out of search indexes, e.g. error pages.
	NoIndex bool
	// Locale is the page's language; the zero value is the default locale.
	Locale locale.Locale
	// Alternates are hreflang links to the page in each language.
	Alternates []Alternate

	body             template.HTML
	siteHeader       template.HTML
	siteFooter       template.HTML
	head             template.HTML
	scriptURL        string
	resolveRelations []string
	storyID          int64
	devToolbar       *DevToolbar
}

// DevToolbar links rendered blocks to the Visual Editor during local
// development.
type DevToolbar struct {
	SpaceID int64
	StoryID int64
}

// NewPage prepares a story for rendering. Content types that implement
// Metadata() provide the document title and description.
func NewPage(story storyblok.Story[AnyBlock]) Page {
	page := Page{Metadata: Metadata{Title: story.Name}, Content: story.Content.Block, storyID: story.ID}
	if m, ok := story.Content.Block.(interface{ Metadata() Metadata }); ok {
		meta := m.Metadata()
		meta.Title = cmp.Or(meta.Title, story.Name)
		page.Metadata = meta
	}
	return page
}

func (p Page) Body() template.HTML        { return p.body }
func (p Page) Head() template.HTML        { return p.head }
func (p Page) SiteHeader() template.HTML  { return p.siteHeader }
func (p Page) SiteFooter() template.HTML  { return p.siteFooter }
func (p Page) ScriptURL() string          { return p.scriptURL }
func (p Page) ResolveRelations() []string { return p.resolveRelations }
func (p Page) DevToolbar() *DevToolbar    { return p.devToolbar }

// HTMXURL is the self-hosted htmx build. The version in the file name makes
// it cacheable forever.
const HTMXURL = "/assets/vendor/htmx-4.0.0.min.js"

func (p Page) HTMXURL() string { return HTMXURL }

// Alternate is the page in one language. OG is empty for x-default.
type Alternate struct {
	Lang, Href, OG string
}

func (p Page) Lang() string     { return cmp.Or(p.Locale.Code, locale.Default.Code) }
func (p Page) OGLocale() string { return cmp.Or(p.Locale.OG, locale.Default.OG) }

// FontURL is the self-hosted Inter variable font (Latin subset, all weights).
const FontURL = "/assets/vendor/inter-4.1-latin.woff2"

func (p Page) FontURL() string { return FontURL }

// WebVitalsURL is the self-hosted web-vitals build, versioned like HTMXURL.
const WebVitalsURL = "/assets/vendor/web-vitals-6.2.3.iife.js"

func (p Page) WebVitalsURL() string { return WebVitalsURL }

type componentAssets struct {
	css, js string
}

type Renderer struct {
	templates  *template.Template
	assets     map[string]componentAssets
	globalCSS  string
	script     []byte
	scriptHash string
	devSpaceID int64
	// idleRenders holds finished renders for reuse. Unlike a sync.Pool, it
	// keeps them across garbage collections, which run about once per page
	// under load, so each template clone escapes its templates only once.
	idleRenders chan *render
}

// maxIdleRenders bounds the template clones kept for reuse; renders beyond it
// clone the templates again.
const maxIdleRenders = 64

type RendererOption func(*Renderer)

// WithDevToolbar marks every block with its UID and adds a toolbar that opens
// the clicked block in the Visual Editor of the given space.
func WithDevToolbar(spaceID int64) RendererOption {
	return func(r *Renderer) { r.devSpaceID = spaceID }
}

func NewRenderer(opts ...RendererOption) (*Renderer, error) {
	if _, err := Schemas(); err != nil {
		return nil, err
	}
	r := &Renderer{}
	for _, opt := range opts {
		opt(r)
	}
	// Each render replaces the functions with ones bound to its state.
	templates, err := template.New("").Funcs((&render{}).funcs()).ParseFS(templateFS, "*.html")
	if err != nil {
		return nil, err
	}
	r.templates = templates
	r.idleRenders = make(chan *render, maxIdleRenders)
	for name := range registry {
		if templates.Lookup(name) == nil {
			return nil, fmt.Errorf("%s: missing named HTML template", name)
		}
	}
	if err := r.loadAssets(); err != nil {
		return nil, err
	}
	return r, nil
}

const globalStylesheet = "base.css"

// loadAssets reads the component styles, which pages inline in <head> once
// per used component type, and bundles the component scripts. Inlining saves
// the render-blocking request a stylesheet costs on first visits, while a
// page's CSS fits the first round trip; see GAPS.md for measurements.
func (r *Renderer) loadAssets() error {
	names, err := fs.Glob(assetFS, "*")
	if err != nil {
		return err
	}
	slices.Sort(names)
	r.assets = map[string]componentAssets{}
	var js bytes.Buffer
	for _, name := range names {
		data, err := assetFS.ReadFile(name)
		if err != nil {
			return err
		}
		if name == globalStylesheet {
			r.globalCSS = string(data)
			continue
		}
		component := strings.TrimSuffix(name, path.Ext(name))
		a := r.assets[component]
		switch path.Ext(name) {
		case ".css":
			a.css = string(data)
		case ".js":
			a.js = string(data)
			// Each script gets its own scope, so top-level names can't clash.
			fmt.Fprintf(&js, "// %s\n(() => {\n%s})();\n", name, data)
		}
		r.assets[component] = a
	}
	r.script = js.Bytes()
	sum := sha256.Sum256(r.script)
	r.scriptHash = hex.EncodeToString(sum[:8])
	return nil
}

// Script returns all component scripts.
func (r *Renderer) Script() []byte { return r.script }

// Page renders a full HTML document.
func (r *Renderer) Page(w io.Writer, page Page) error {
	rn := r.newRender(page.Locale)
	defer r.releaseRender(rn)
	body, err := rn.block(page.Content)
	if err != nil {
		return err
	}
	page.body = body
	if page.Chrome != nil && page.Chrome.Settings != nil {
		rn.chrome = true
		if page.siteHeader, err = rn.component("site-header", "site-settings", *page.Chrome); err != nil {
			return err
		}
		if page.siteFooter, err = rn.component("site-footer", "site-settings", *page.Chrome); err != nil {
			return err
		}
	}
	page.resolveRelations = ResolveRelations
	if len(r.script) > 0 {
		page.scriptURL = "/assets/app.js?v=" + r.scriptHash
	}
	var css strings.Builder
	css.WriteString(r.globalCSS)
	for _, name := range rn.used {
		css.WriteString(r.assets[name].css)
	}
	page.head = styleElement(css.String())
	// Inside the Visual Editor the bridge already makes blocks clickable.
	if r.devSpaceID != 0 && !page.Preview {
		page.devToolbar = &DevToolbar{SpaceID: r.devSpaceID, StoryID: page.storyID}
	}
	return rn.templates.ExecuteTemplate(w, "layout", page)
}

// Block renders a single block, such as a story's content type.
func (r *Renderer) Block(w io.Writer, block Block, loc locale.Locale) error {
	rn := r.newRender(loc)
	defer r.releaseRender(rn)
	html, err := rn.block(block)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, string(html))
	return err
}

// Fragment renders the part of a block that an enhanced request replaces:
// the "<component>-fragment" template if the component defines one, else the
// whole block. Fragments update components their page already rendered, so
// their CSS is in place.
func (r *Renderer) Fragment(w io.Writer, block Block, loc locale.Locale) error {
	name := block.Meta().Component + "-fragment"
	if r.templates.Lookup(name) == nil {
		return r.Block(w, block, loc)
	}
	rn := r.newRender(loc)
	defer r.releaseRender(rn)
	html, err := rn.component(name, block.Meta().Component, block)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, string(html))
	return err
}

// render holds the state of one rendering pass.
type render struct {
	*Renderer
	templates *template.Template
	used      []string
	// firstSection is set while rendering a page's first section, whose
	// images are likely the Largest Contentful Paint.
	firstSection bool
	// chrome is set while rendering the site header and footer. Their blocks
	// belong to the settings story, so the Visual Editor and dev toolbar must
	// not treat them as the page's blocks.
	chrome bool
	locale locale.Locale
}

func (r *Renderer) newRender(loc locale.Locale) *render {
	var rn *render
	select {
	case rn = <-r.idleRenders:
	default:
		rn = &render{Renderer: r}
		// Each clone binds the template functions to its own render. Reusing
		// clones keeps html/template from escaping templates every request.
		rn.templates = template.Must(r.templates.Clone()).Funcs(rn.funcs())
	}
	rn.locale = loc
	if rn.locale.Code == "" {
		rn.locale = locale.Default
	}
	rn.used = rn.used[:0]
	rn.firstSection = false
	rn.chrome = false
	return rn
}

func (r *Renderer) releaseRender(rn *render) {
	select {
	case r.idleRenders <- rn:
	default:
	}
}

func (rn *render) funcs() template.FuncMap {
	return template.FuncMap{
		"render":    rn.block,
		"renderAll": rn.blocks,
		"markdown":  renderMarkdown,
		"t":         func(key string, args ...any) string { return rn.locale.T(key, args...) },
		"number":    func(n int) string { return rn.locale.FormatNumber(n) },
		"section": func(block Block) string {
			rn.markUsed("base-section")
			return sectionClass(block)
		},
		"renderSections": rn.sections,
		"firstSection":   func() bool { return rn.firstSection },
		"base":           rn.base,
		"editable":       rn.editableAttrs,
		"id":             ElementID,
		"shortID":        ShortID,
		"join":           strings.Join,
		"contains":       slices.Contains[[]string],
	}
}

func (rn *render) block(block Block) (template.HTML, error) {
	if block == nil {
		return "", nil
	}
	name := block.Meta().Component
	if _, unknown := block.(*Unknown); unknown || rn.templates.Lookup(name) == nil {
		name = "unknown"
	}
	return rn.component(name, name, block)
}

func (rn *render) blocks(blocks Blocks) (template.HTML, error) {
	var buf strings.Builder
	for _, block := range blocks {
		html, err := rn.block(block)
		if err != nil {
			return "", err
		}
		buf.WriteString(string(html))
	}
	return template.HTML(buf.String()), nil
}

// sections renders a page's sections and marks the first one, so its images
// load with priority.
func (rn *render) sections(blocks Blocks) (template.HTML, error) {
	defer func() { rn.firstSection = false }()
	var buf strings.Builder
	for i, block := range blocks {
		rn.firstSection = i == 0
		html, err := rn.block(block)
		if err != nil {
			return "", err
		}
		buf.WriteString(string(html))
	}
	return template.HTML(buf.String()), nil
}

// base renders a base component, such as {{base "base-card" .Card}}.
func (rn *render) base(name string, data any) (template.HTML, error) {
	if !strings.HasPrefix(name, "base-") {
		return "", fmt.Errorf("%s is not a base component", name)
	}
	return rn.component(name, name, data)
}

func (rn *render) markUsed(component string) {
	if !slices.Contains(rn.used, component) {
		rn.used = append(rn.used, component)
	}
}

// component executes a template and records the component whose styles the
// page needs.
func (rn *render) component(templateName, component string, data any) (template.HTML, error) {
	rn.markUsed(component)
	var buf strings.Builder
	if err := rn.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func styleElement(css string) template.HTML {
	if css == "" {
		return ""
	}
	return template.HTML("<style>" + css + "</style>")
}

// ElementID derives a document-unique ID from a block's short ID, which can
// start with a digit, which CSS ID selectors don't allow.
func ElementID(block Block) string { return "b-" + ShortID(block) }

// editableAttrs returns the attributes the Visual Editor uses to make a blok
// clickable. Published content has no editable marker, so this is empty
// unless the dev toolbar needs the blok's UID.
func (rn *render) editableAttrs(block Block) template.HTMLAttr {
	if block == nil || rn.chrome {
		return ""
	}
	meta := block.Meta()
	options, uid, ok := meta.EditableOptions()
	if !ok {
		if rn.devSpaceID == 0 || meta.UID == "" {
			return ""
		}
		return template.HTMLAttr(`data-dev-blok="` + template.HTMLEscapeString(meta.UID) +
			`" data-dev-component="` + template.HTMLEscapeString(meta.Component) + `"`)
	}
	return template.HTMLAttr(`data-blok-c="` + template.HTMLEscapeString(options) +
		`" data-blok-uid="` + template.HTMLEscapeString(uid) + `"`)
}
