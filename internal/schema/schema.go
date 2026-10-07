// Package schema defines CMS schemas alongside their Go content models.
package schema

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

type Category string

const (
	Page    Category = "page"
	Section Category = "block-section"
	Media   Category = "block-media"
	Content Category = "block-content"
)

// Field is an authoring definition. Order in Fields determines editor order.
type Field struct {
	Name        string
	Type        string
	Label       string
	Description string
	Required    bool
	Allow       Category
}

type Definition[T any] struct {
	Name        string
	DisplayName string
	Category    Category
	Fields      []Field
}

// Component is the writable portion of the MAPI component object we own.
// Space-specific IDs and editor metadata (folders, icons, previews) stay remote.
type Component struct {
	Name        string                    `json:"name"`
	DisplayName string                    `json:"display_name"`
	IsRoot      bool                      `json:"is_root"`
	IsNestable  bool                      `json:"is_nestable"`
	Schema      map[string]map[string]any `json:"schema"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`)

func (d Definition[T]) Validate() error {
	if !validCategory(d.Category) || !strings.HasPrefix(d.Name, string(d.Category)+"-") || !identifier.MatchString(d.Name) {
		return fmt.Errorf("invalid component name/category: %q (%s)", d.Name, d.Category)
	}
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("%s: model must be a struct", d.Name)
	}
	model := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type == reflect.TypeFor[storyblok.Blok]() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if !f.IsExported() || name == "" {
			return fmt.Errorf("%s.%s: content fields require an explicit JSON name", d.Name, f.Name)
		}
		if _, exists := model[name]; exists {
			return fmt.Errorf("%s: duplicate JSON field %s", d.Name, name)
		}
		model[name] = f.Type
	}
	seen := map[string]bool{}
	for _, f := range d.Fields {
		if !identifier.MatchString(f.Name) || seen[f.Name] {
			return fmt.Errorf("%s: invalid or duplicate schema field %q", d.Name, f.Name)
		}
		seen[f.Name] = true
		ft, ok := model[f.Name]
		if !ok {
			return fmt.Errorf("%s.%s: schema field missing from Go model", d.Name, f.Name)
		}
		switch f.Type {
		case "text", "textarea":
			if ft.Kind() != reflect.String || f.Allow != "" {
				return fmt.Errorf("%s.%s: text requires a string and no block restrictions", d.Name, f.Name)
			}
		case "bloks":
			meta := reflect.TypeFor[interface{ Meta() *storyblok.Blok }]()
			if ft.Kind() != reflect.Slice || !ft.Elem().Implements(meta) || !validCategory(f.Allow) || f.Allow == Page {
				return fmt.Errorf("%s.%s: blocks require a slice of blocks and a nestable category", d.Name, f.Name)
			}
			if d.Category == Page && f.Allow != Section {
				return fmt.Errorf("%s.%s: pages only accept sections", d.Name, f.Name)
			}
		default:
			return fmt.Errorf("%s.%s: unsupported field type %q", d.Name, f.Name, f.Type)
		}
	}
	for name := range model {
		if !seen[name] {
			return fmt.Errorf("%s.%s: Go content field has no schema", d.Name, name)
		}
	}
	return nil
}

func validCategory(c Category) bool { return c == Page || c == Section || c == Media || c == Content }

// Compile resolves categories against the complete registry, never wildcards.
func (d Definition[T]) Compile(categories map[string]Category) (Component, error) {
	if err := d.Validate(); err != nil {
		return Component{}, err
	}
	c := Component{Name: d.Name, DisplayName: d.DisplayName, IsRoot: d.Category == Page, IsNestable: d.Category != Page, Schema: map[string]map[string]any{}}
	for pos, f := range d.Fields {
		field := map[string]any{"type": f.Type, "pos": pos, "display_name": f.Label, "description": f.Description, "required": f.Required}
		if f.Type == "bloks" {
			allowed := []string{}
			for name, category := range categories {
				if category == f.Allow {
					allowed = append(allowed, name)
				}
			}
			slices.Sort(allowed)
			if len(allowed) == 0 {
				return Component{}, fmt.Errorf("%s.%s: no registered %s components", d.Name, f.Name, f.Allow)
			}
			field["restrict_components"] = true
			field["component_whitelist"] = allowed
		}
		c.Schema[f.Name] = field
	}
	return c, nil
}
