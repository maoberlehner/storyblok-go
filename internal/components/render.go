package components

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io"
	"io/fs"
	"slices"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

var (
	//go:embed *.html
	templateFS embed.FS
	//go:embed *.css
	styleFS embed.FS
)

// ResolveRelations lists the relation fields components expect as resolved
// stories, both from the Content Delivery API and the Visual Editor bridge.
var ResolveRelations = []string{
	"quotes_item.quote",
	"section_with_quote_card.quote",
	"hero_with_image_ctas.quote",
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
	Config  *Configuration
	// Preview loads the Visual Editor bridge and live preview script.
	Preview bool

	stylesheetURL    string
	resolveRelations []string
}

// NewPage prepares a story for rendering. Content types that implement
// Metadata() provide the document title and description.
func NewPage(story storyblok.Story[AnyBlock]) Page {
	page := Page{Metadata: Metadata{Title: story.Name}, Content: story.Content.Block}
	if m, ok := story.Content.Block.(interface{ Metadata() Metadata }); ok {
		meta := m.Metadata()
		meta.Title = cmp.Or(meta.Title, story.Name)
		page.Metadata = meta
	}
	return page
}

func (p Page) StylesheetURL() string      { return p.stylesheetURL }
func (p Page) ResolveRelations() []string { return p.resolveRelations }

type Renderer struct {
	templates      *template.Template
	stylesheet     []byte
	stylesheetHash string
}

func NewRenderer() (*Renderer, error) {
	r := &Renderer{}
	templates, err := template.New("").Funcs(template.FuncMap{
		"render":         r.renderBlock,
		"renderAll":      r.renderBlocks,
		"editable":       editableAttrs,
		"richtext":       r.richtext,
		"richtextInline": r.richtextInline,
		"join":           strings.Join,
		"contains":       slices.Contains[[]string],
	}).ParseFS(templateFS, "*.html")
	if err != nil {
		return nil, err
	}
	r.templates = templates

	if r.stylesheet, err = bundleStylesheets(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(r.stylesheet)
	r.stylesheetHash = hex.EncodeToString(sum[:8])
	return r, nil
}

// Page renders a full HTML document.
func (r *Renderer) Page(w io.Writer, page Page) error {
	page.stylesheetURL = "/assets/app.css?v=" + r.stylesheetHash
	page.resolveRelations = ResolveRelations
	return r.templates.ExecuteTemplate(w, "layout", page)
}

// Block renders a single block, such as a story's content type.
func (r *Renderer) Block(w io.Writer, block Block) error {
	html, err := r.renderBlock(block)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, string(html))
	return err
}

// Stylesheet returns base.css followed by all component stylesheets.
func (r *Renderer) Stylesheet() []byte { return r.stylesheet }

func (r *Renderer) renderBlock(block Block) (template.HTML, error) {
	if block == nil {
		return "", nil
	}
	name := block.Meta().Component
	if _, unknown := block.(*Unknown); unknown || r.templates.Lookup(name) == nil {
		name = "unknown"
	}
	var buf strings.Builder
	if err := r.templates.ExecuteTemplate(&buf, name, block); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func (r *Renderer) renderBlocks(blocks Blocks) (template.HTML, error) {
	var buf strings.Builder
	for _, block := range blocks {
		html, err := r.renderBlock(block)
		if err != nil {
			return "", err
		}
		buf.WriteString(string(html))
	}
	return template.HTML(buf.String()), nil
}

// editableAttrs returns the attributes the Visual Editor uses to make a blok
// clickable. Published content has no editable marker, so this is empty.
func editableAttrs(block Block) template.HTMLAttr {
	if block == nil {
		return ""
	}
	options, uid, ok := block.Meta().EditableOptions()
	if !ok {
		return ""
	}
	return template.HTMLAttr(`data-blok-c="` + template.HTMLEscapeString(options) +
		`" data-blok-uid="` + template.HTMLEscapeString(uid) + `"`)
}

const baseStylesheet = "base.css"

func bundleStylesheets() ([]byte, error) {
	names, err := fs.Glob(styleFS, "*.css")
	if err != nil {
		return nil, err
	}
	slices.SortFunc(names, func(a, b string) int {
		switch {
		case a == baseStylesheet:
			return -1
		case b == baseStylesheet:
			return 1
		default:
			return strings.Compare(a, b)
		}
	})
	var buf bytes.Buffer
	for _, name := range names {
		css, err := styleFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		buf.WriteString("/* " + name + " */\n")
		buf.Write(css)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}
