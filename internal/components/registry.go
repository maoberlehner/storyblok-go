// Package components renders Storyblok bloks to HTML. Each component is a Go
// struct, an html/template definition named after the component, and a
// stylesheet, kept side by side as <component>.{go,html,css}.
package components

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"storyblok-go-website/internal/storyblok"
)

// Block is a decoded blok of any component.
type Block interface {
	Meta() *storyblok.Blok
}

var registry = map[string]func() Block{}

// register makes a component decodable. Call it from an init function in the
// component's file.
func register[T any, P interface {
	*T
	Block
}](name string) {
	registry[name] = func() Block { return P(new(T)) }
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
	return block
}
