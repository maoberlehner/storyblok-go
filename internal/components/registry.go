// Package components renders Storyblok bloks to HTML. Each component is a Go
// struct, an html/template definition named after the component, and a
// stylesheet, and mandatory schema, kept side by side.
package components

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

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

// register makes a component decodable. Call it from an init function in the
// component's .schema.go file.
func register[T any, P interface {
	*T
	Block
}](definition schema.Definition[T]) {
	name := definition.Name
	if _, exists := registry[name]; exists {
		panic("duplicate component: " + name)
	}
	registry[name] = func() Block { return P(new(T)) }
	categories[name] = definition.Category
	definitions[name] = definition.Compile
}

// IsPage reports whether component is a registered page content type.
func IsPage(component string) bool { return categories[component] == schema.Page }

// Schemas validates every CMS component and resolves its allowed children.
// Registration requires a schema definition; base templates are not registered.
func Schemas() ([]schema.Component, error) {
	files, err := templateFS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		name := strings.TrimSuffix(file.Name(), ".html")
		if strings.HasPrefix(name, "page-") || strings.HasPrefix(name, "block-") {
			if _, ok := definitions[name]; !ok {
				return nil, fmt.Errorf("%s: CMS template has no registered schema", name)
			}
		}
	}
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
		if _, err := templateFS.ReadFile(name + ".html"); err != nil {
			return nil, fmt.Errorf("%s: missing rendering template: %w", name, err)
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
