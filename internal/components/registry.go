// Package components renders Storyblok bloks to HTML. Each component is a Go
// struct, a templ view, a stylesheet, and for CMS components a schema, kept
// side by side.
package components

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"storyblok-go-website/internal/schema"
	"storyblok-go-website/internal/storyblok"
)

// Block is a decoded blok of any component.
type Block interface {
	Meta() *storyblok.Blok
}

var registry = map[string]func() Block{}
var categories = map[string]schema.Category{}
var definitions = map[string]func(map[string]schema.Category) (schema.Component, error){}
var views = map[string]func(Block) templ.Component{}

// register makes a component decodable and renderable. Call it from an init
// function in the component's .schema.go file.
// The view takes the block, or for loaders the type their Load returns.
func register[T any, P interface {
	*T
	Block
}, V Block](definition schema.Definition[T], view func(V) templ.Component) {
	name := definition.Name
	if _, exists := registry[name]; exists {
		panic("duplicate component: " + name)
	}
	registry[name] = func() Block { return P(new(T)) }
	categories[name] = definition.Category
	definitions[name] = definition.Compile
	registerView(name, view)
}

// registerView renders blocks of a component, including built-in ones that
// never come from the CMS.
func registerView[V Block](name string, view func(V) templ.Component) {
	views[name] = func(block Block) templ.Component {
		v, ok := block.(V)
		if !ok {
			return errorComponent(fmt.Errorf("%s: cannot render %T; is it loaded?", name, block))
		}
		return view(v)
	}
}

func errorComponent(err error) templ.Component {
	return templ.ComponentFunc(func(context.Context, io.Writer) error { return err })
}

// IsPage reports whether component is a registered page content type.
func IsPage(component string) bool { return categories[component] == schema.Page }

// Schemas validates every CMS component and resolves its allowed children.
func Schemas() ([]schema.Component, error) {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]schema.Component, 0, len(names))
	for _, name := range names {
		component, err := definitions[name](categories)
		if err != nil {
			return nil, err
		}
		result = append(result, component)
	}
	return result, nil
}

// Unknown stands in for bloks that have no registered component or failed to
// decode, so one broken blok does not take down the page.
type Unknown struct {
	storyblok.Blok
	Err error
}

// Blocks is a bloks field.
type Blocks []Block

func (bs *Blocks) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() != '[' {
		return dec.SkipValue()
	}
	var raws []jsontext.Value
	if err := json.UnmarshalDecode(dec, &raws); err != nil {
		return err
	}
	*bs = make(Blocks, 0, len(raws))
	for _, raw := range raws {
		*bs = append(*bs, decodeBlock(raw))
	}
	return nil
}

// AnyBlock is a single blok of any component, such as a story's content.
type AnyBlock struct {
	Block
}

func (a *AnyBlock) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	a.Block = decodeBlock(raw.Clone())
	return nil
}

func decodeBlock(raw jsontext.Value) Block {
	var meta storyblok.Blok
	if err := json.Unmarshal(raw, &meta); err != nil {
		return &Unknown{Err: err}
	}
	newBlock, ok := registry[meta.Component]
	if !ok {
		return &Unknown{Blok: meta}
	}
	block := newBlock()
	if err := json.Unmarshal(raw, block); err != nil {
		return &Unknown{Blok: meta, Err: fmt.Errorf("decoding %s: %w", meta.Component, err)}
	}
	if page, ok := block.(sectioned); ok {
		for _, child := range page.SectionBlocks() {
			if !strings.HasPrefix(child.Meta().Component, "block-section-") {
				// Keep unknown blocks visible to editors, but reject known non-sections.
				if _, unknown := child.(*Unknown); !unknown {
					return &Unknown{Blok: meta, Err: fmt.Errorf("pages only accept sections")}
				}
			}
		}
	}
	return block
}
