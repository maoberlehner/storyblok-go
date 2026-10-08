package components

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
	"storyblok-go-website/static"
)

//go:embed *.css *.js
var assetFS embed.FS

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
	// Loaded holds the request's versions of the page's blocks.
	Loaded Loaded

	body       string
	siteHeader string
	siteFooter string
	css        string
	scriptURL  string
	storyID    int64
	devToolbar *DevToolbar
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

// htmxFile is the self-hosted htmx build in static.FS. The version in the
// file name makes it cacheable forever.
const htmxFile = "vendor/htmx-4.0.0.min.js"

const HTMXURL = "/assets/" + htmxFile

// HTMXIntegrity is the subresource integrity hash of the embedded htmx build,
// so browsers refuse a copy altered on the way, such as by a cache.
var HTMXIntegrity = subresourceIntegrity(static.FS, htmxFile)

func subresourceIntegrity(fsys fs.FS, name string) string {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		panic(err)
	}
	sum := sha512.Sum384(b)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

// Alternate is the page in one language. OG is empty for x-default.
type Alternate struct {
	Lang, Href, OG string
}

func (p Page) Lang() string     { return cmp.Or(p.Locale.Code, locale.Default.Code) }
func (p Page) OGLocale() string { return cmp.Or(p.Locale.OG, locale.Default.OG) }

// FontURL is the self-hosted Inter variable font (Latin subset, all weights).
const FontURL = "/assets/vendor/inter-4.1-latin.woff2"

// WebVitalsURL is the self-hosted web-vitals build, versioned like HTMXURL.
const WebVitalsURL = "/assets/vendor/web-vitals-6.2.3.iife.js"

type componentAssets struct {
	css, js string
}

type Renderer struct {
	assets     map[string]componentAssets
	globalCSS  string
	script     []byte
	scriptHash string
	devSpaceID int64
}

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

// Page renders a full HTML document. The body renders first, so the head
// can inline the styles of exactly the components it used.
func (r *Renderer) Page(w io.Writer, page Page) error {
	rn, ctx := r.newRender(page.Locale, page.Loaded)
	var buf strings.Builder
	if err := renderBlock(page.Content).Render(ctx, &buf); err != nil {
		return err
	}
	page.body = buf.String()
	if page.Chrome != nil && page.Chrome.Settings != nil {
		rn.chrome = true
		rn.markUsed("site-settings")
		buf.Reset()
		if err := siteHeader(*page.Chrome).Render(ctx, &buf); err != nil {
			return err
		}
		page.siteHeader = buf.String()
		buf.Reset()
		if err := siteFooter(*page.Chrome).Render(ctx, &buf); err != nil {
			return err
		}
		page.siteFooter = buf.String()
	}
	if len(r.script) > 0 {
		page.scriptURL = "/assets/app.js?v=" + r.scriptHash
	}
	css, err := r.styles(rn.used)
	if err != nil {
		return err
	}
	page.css = css
	// Inside the Visual Editor the bridge already makes blocks clickable.
	if r.devSpaceID != 0 && !page.Preview {
		page.devToolbar = &DevToolbar{SpaceID: r.devSpaceID, StoryID: page.storyID}
	}
	return layout(page).Render(ctx, w)
}

// styles returns the global styles and those of the given components.
func (r *Renderer) styles(components []string) (string, error) {
	size := len(r.globalCSS)
	for _, name := range components {
		// Every component has a stylesheet, so a missing one is a misspelled
		// name.
		if r.assets[name].css == "" {
			return "", fmt.Errorf("%s: no component stylesheet", name)
		}
		size += len(r.assets[name].css)
	}
	var css strings.Builder
	css.Grow(size)
	css.WriteString(r.globalCSS)
	for _, name := range components {
		css.WriteString(r.assets[name].css)
	}
	return css.String(), nil
}

// Block renders a single block, such as a story's content type.
func (r *Renderer) Block(w io.Writer, block Block, loc locale.Locale, loaded Loaded) error {
	_, ctx := r.newRender(loc, loaded)
	return renderBlock(block).Render(ctx, w)
}

// fragmenter is a block whose enhanced requests replace only part of it.
type fragmenter interface {
	Fragment() templ.Component
}

// Fragment renders the part of a block that an enhanced request replaces:
// the block's Fragment if it has one, else the whole block. Fragments update
// components their page already rendered, so their CSS is in place.
func (r *Renderer) Fragment(w io.Writer, block Block, loc locale.Locale) error {
	f, ok := block.(fragmenter)
	if !ok {
		return r.Block(w, block, loc, nil)
	}
	_, ctx := r.newRender(loc, nil)
	return f.Fragment().Render(ctx, w)
}

// render holds the state of one rendering pass. Components reach it through
// their context.
type render struct {
	*Renderer
	used []string
	// firstSection is set while rendering a page's first section, whose
	// images are likely the Largest Contentful Paint.
	firstSection bool
	// chrome is set while rendering the site header and footer. Their blocks
	// belong to the settings story, so the Visual Editor and dev toolbar must
	// not treat them as the page's blocks.
	chrome bool
	locale locale.Locale
	loaded Loaded
}

type renderKey struct{}

func (r *Renderer) newRender(loc locale.Locale, loaded Loaded) (*render, context.Context) {
	if loc.Code == "" {
		loc = locale.Default
	}
	rn := &render{Renderer: r, locale: loc, loaded: loaded}
	return rn, context.WithValue(context.Background(), renderKey{}, rn)
}

func renderState(ctx context.Context) *render { return ctx.Value(renderKey{}).(*render) }

// markUsed records a component whose styles the page needs.
func (rn *render) markUsed(component string) {
	if !slices.Contains(rn.used, component) {
		rn.used = append(rn.used, component)
	}
}

// renderBlock renders a block with the view registered for its component.
func renderBlock(block Block) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if block == nil {
			return nil
		}
		if view, ok := renderState(ctx).loaded[block]; ok {
			block = view
		}
		name := block.Meta().Component
		view, ok := views[name]
		if u, unknown := block.(*Unknown); unknown {
			name, view, ok = "unknown", func(Block) templ.Component { return unknownBlock(u) }, true
		}
		if !ok {
			return fmt.Errorf("%s: no registered view", name)
		}
		renderState(ctx).markUsed(name)
		return view(block).Render(ctx, w)
	})
}

func renderBlocks(blocks Blocks) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, block := range blocks {
			if err := renderBlock(block).Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}

// renderSections renders a page's sections and marks the first one, so its
// images load with priority.
func renderSections(blocks Blocks) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		rn := renderState(ctx)
		defer func() { rn.firstSection = false }()
		for i, block := range blocks {
			rn.firstSection = i == 0
			if err := renderBlock(block).Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}

// styled renders a base component's markup, its children, and records that
// the page needs its styles.
func styled(component string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		renderState(ctx).markUsed(component)
		return templ.GetChildren(ctx).Render(ctx, w)
	})
}

// t translates a UI string into the language of the page.
func t(ctx context.Context, key string, args ...any) string {
	return renderState(ctx).locale.T(key, args...)
}

func number(ctx context.Context, n int) string { return renderState(ctx).locale.FormatNumber(n) }

func isFirstSection(ctx context.Context) bool { return renderState(ctx).firstSection }

// section returns the class list of a section's root element.
func section(ctx context.Context, block Block) string {
	renderState(ctx).markUsed("base-section")
	return sectionClass(block)
}

// ElementID derives a document-unique ID from a block's short ID, which can
// start with a digit, which CSS ID selectors don't allow.
func ElementID(block Block) string { return "b-" + ShortID(block) }

// editable returns the attributes the Visual Editor uses to make a blok
// clickable. Published content has no editable marker, so this is empty
// unless the dev toolbar needs the blok's UID.
func editable(ctx context.Context, block Block) templ.Attributer {
	rn := renderState(ctx)
	if block == nil || rn.chrome {
		return templ.OrderedAttributes{}
	}
	meta := block.Meta()
	options, uid, ok := meta.EditableOptions()
	if !ok {
		if rn.devSpaceID == 0 || meta.UID == "" {
			return templ.OrderedAttributes{}
		}
		return templ.OrderedAttributes{{Key: "data-dev-blok", Value: meta.UID}, {Key: "data-dev-component", Value: meta.Component}}
	}
	return templ.OrderedAttributes{{Key: "data-blok-c", Value: options}, {Key: "data-blok-uid", Value: uid}}
}
