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
	"sync"

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

// AssetDelivery selects how component CSS and JS reach the browser. All modes
// are kept until a benchmark settles the choice (see GAPS.md).
type AssetDelivery string

const (
	// DeliverInline puts each component's CSS and JS next to every instance.
	DeliverInline AssetDelivery = "inline"
	// DeliverHead collects the CSS of used components in <head>, once per
	// component type. JS is delivered like DeliverInline.
	DeliverHead AssetDelivery = "head"
	// DeliverBundle links one stylesheet and one script for all components.
	DeliverBundle AssetDelivery = "bundle"
)

func ParseAssetDelivery(s string) (AssetDelivery, error) {
	switch mode := AssetDelivery(cmp.Or(s, string(DeliverBundle))); mode {
	case DeliverInline, DeliverHead, DeliverBundle:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown asset delivery %q", s)
	}
}

type Metadata struct {
	Title       string
	Description string
	Image       storyblok.Asset
}

// Page is everything the layout needs to render a full document.
type Page struct {
	Metadata
	Content Block
	// Preview loads the Visual Editor bridge and live preview script.
	Preview bool

	body             template.HTML
	head             template.HTML
	stylesheetURL    string
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
func (p Page) StylesheetURL() string      { return p.stylesheetURL }
func (p Page) ScriptURL() string          { return p.scriptURL }
func (p Page) ResolveRelations() []string { return p.resolveRelations }
func (p Page) DevToolbar() *DevToolbar    { return p.devToolbar }

// HTMXURL is the self-hosted htmx build. The version in the file name makes
// it cacheable forever.
const HTMXURL = "/assets/vendor/htmx-4.0.0.min.js"

func (p Page) HTMXURL() string { return HTMXURL }

type componentAssets struct {
	css, js string
}

type Renderer struct {
	templates  *template.Template
	delivery   AssetDelivery
	assets     map[string]componentAssets
	globalCSS  string
	stylesheet []byte
	script     []byte
	assetHash  string
	devSpaceID int64
	renders    sync.Pool
}

type RendererOption func(*Renderer)

// WithDevToolbar marks every block with its UID and adds a toolbar that opens
// the clicked block in the Visual Editor of the given space.
func WithDevToolbar(spaceID int64) RendererOption {
	return func(r *Renderer) { r.devSpaceID = spaceID }
}

func WithAssetDelivery(mode AssetDelivery) RendererOption {
	return func(r *Renderer) { r.delivery = mode }
}

func NewRenderer(opts ...RendererOption) (*Renderer, error) {
	if _, err := Schemas(); err != nil {
		return nil, err
	}
	r := &Renderer{delivery: DeliverBundle}
	for _, opt := range opts {
		opt(r)
	}
	// Pooled renders replace the functions with ones bound to their state.
	templates, err := template.New("").Funcs((&render{}).funcs()).ParseFS(templateFS, "*.html")
	if err != nil {
		return nil, err
	}
	r.templates = templates
	r.renders.New = func() any {
		rn := &render{Renderer: r}
		// Each pooled clone binds the template functions to its own state.
		// Reusing clones keeps html/template from escaping every request.
		rn.templates = template.Must(r.templates.Clone()).Funcs(rn.funcs())
		return rn
	}
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

func (r *Renderer) loadAssets() error {
	names, err := fs.Glob(assetFS, "*")
	if err != nil {
		return err
	}
	slices.Sort(names)
	r.assets = map[string]componentAssets{}
	var css, js bytes.Buffer
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
			fmt.Fprintf(&css, "/* %s */\n%s\n", name, data)
		case ".js":
			a.js = string(data)
			fmt.Fprintf(&js, "// %s\n%s\n", name, data)
		}
		r.assets[component] = a
	}
	r.stylesheet = append([]byte("/* "+globalStylesheet+" */\n"+r.globalCSS+"\n"), css.Bytes()...)
	r.script = js.Bytes()
	sum := sha256.Sum256(append(slices.Clone(r.stylesheet), r.script...))
	r.assetHash = hex.EncodeToString(sum[:8])
	return nil
}

// Stylesheet returns base.css followed by all component stylesheets.
func (r *Renderer) Stylesheet() []byte { return r.stylesheet }

// Script returns all component scripts.
func (r *Renderer) Script() []byte { return r.script }

// Page renders a full HTML document.
func (r *Renderer) Page(w io.Writer, page Page) error {
	rn := r.newRender()
	defer r.renders.Put(rn)
	body, err := rn.block(page.Content)
	if err != nil {
		return err
	}
	page.body = body
	page.resolveRelations = ResolveRelations
	switch r.delivery {
	case DeliverBundle:
		page.stylesheetURL = "/assets/app.css?v=" + r.assetHash
		if len(r.script) > 0 {
			page.scriptURL = "/assets/app.js?v=" + r.assetHash
		}
	case DeliverHead:
		var css strings.Builder
		css.WriteString(r.globalCSS)
		for _, name := range rn.used {
			css.WriteString(r.assets[name].css)
		}
		page.head = styleElement(css.String())
	case DeliverInline:
		page.head = styleElement(r.globalCSS)
	}
	// Inside the Visual Editor the bridge already makes blocks clickable.
	if r.devSpaceID != 0 && !page.Preview {
		page.devToolbar = &DevToolbar{SpaceID: r.devSpaceID, StoryID: page.storyID}
	}
	return rn.templates.ExecuteTemplate(w, "layout", page)
}

// Block renders a single block, such as a story's content type.
func (r *Renderer) Block(w io.Writer, block Block) error {
	rn := r.newRender()
	defer r.renders.Put(rn)
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
// with DeliverHead their CSS is in place and not repeated.
func (r *Renderer) Fragment(w io.Writer, block Block) error {
	name := block.Meta().Component + "-fragment"
	if r.templates.Lookup(name) == nil {
		return r.Block(w, block)
	}
	rn := r.newRender()
	defer r.renders.Put(rn)
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
}

func (r *Renderer) newRender() *render {
	rn := r.renders.Get().(*render)
	rn.used = rn.used[:0]
	return rn
}

func (rn *render) funcs() template.FuncMap {
	return template.FuncMap{
		"render":    rn.block,
		"renderAll": rn.blocks,
		"base":      rn.base,
		"editable":  rn.editableAttrs,
		"id":        ElementID,
		"shortID":   ShortID,
		"join":      strings.Join,
		"contains":  slices.Contains[[]string],
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

// base renders a base component, such as {{base "base-card" .Card}}.
func (rn *render) base(name string, data any) (template.HTML, error) {
	if !strings.HasPrefix(name, "base-") {
		return "", fmt.Errorf("%s is not a base component", name)
	}
	return rn.component(name, name, data)
}

// component executes a template and adds the CSS and JS of the component it
// belongs to as the delivery mode requires.
func (rn *render) component(templateName, component string, data any) (template.HTML, error) {
	var buf strings.Builder
	if !slices.Contains(rn.used, component) {
		rn.used = append(rn.used, component)
	}
	assets := rn.assets[component]
	if rn.delivery == DeliverInline {
		buf.WriteString(string(styleElement(assets.css)))
	}
	if err := rn.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", err
	}
	if rn.delivery != DeliverBundle && assets.js != "" {
		buf.WriteString("<script>" + assets.js + "</script>")
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
	if block == nil {
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
