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
	// Site components hold site-wide data, such as navigation. They are never
	// routed or listed as pages.
	Site Category = "site"
)

// Field is an authoring definition. Order in Fields determines editor order.
type Field struct {
	Name        string
	Type        string
	Label       string
	Description string
	Required    bool
	// Allow restricts a bloks field to a category; Components restricts it to
	// exact components instead.
	Allow      Category
	Components []string
	// Max limits the number of blocks; zero means no limit.
	Max int
	// Options and Default configure option fields. An empty value decodes as
	// Default, which Go code applies.
	Options []Option
	Default string
	// Filetypes restricts asset fields, e.g. "images".
	Filetypes []string
	// NotTranslatable opts text, textarea, and markdown fields out of
	// field-level translation.
	NotTranslatable bool
	// Fields are the members of a "section" field: a collapsible group in the
	// editor that holds no content itself.
	Fields []Field
}

type Option struct {
	Value, Label string
}

type Definition[T any] struct {
	Name        string
	DisplayName string
	Category    Category
	// ContentType makes a site component a story content type, such as the
	// site settings.
	ContentType bool
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

const groupType = "section"

func (d Definition[T]) Validate() error {
	if !validCategory(d.Category) || !strings.HasPrefix(d.Name, string(d.Category)+"-") || !identifier.MatchString(d.Name) {
		return fmt.Errorf("invalid component name/category: %q (%s)", d.Name, d.Category)
	}
	if d.ContentType && d.Category != Site {
		return fmt.Errorf("%s: only site components can be content types", d.Name)
	}
	if d.Category == Page {
		if err := d.validatePageMetadata(); err != nil {
			return err
		}
	}
	model, err := modelFields(d.Name, reflect.TypeFor[T]())
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, f := range d.Fields {
		if f.Type != groupType {
			if err := d.validateField(f, model, seen); err != nil {
				return err
			}
			continue
		}
		if !identifier.MatchString(f.Name) || seen[f.Name] {
			return fmt.Errorf("%s: invalid or duplicate schema field %q", d.Name, f.Name)
		}
		seen[f.Name] = true
		if len(f.Fields) == 0 {
			return fmt.Errorf("%s.%s: empty group", d.Name, f.Name)
		}
		for _, member := range f.Fields {
			if member.Type == groupType {
				return fmt.Errorf("%s.%s: groups cannot be nested", d.Name, member.Name)
			}
			if err := d.validateField(member, model, seen); err != nil {
				return err
			}
		}
	}
	for name := range model {
		if !seen[name] {
			return fmt.Errorf("%s.%s: Go content field has no schema", d.Name, name)
		}
	}
	return nil
}

// modelFields maps JSON names to Go types. Embedded structs without a JSON
// name contribute their fields, as they do when decoding.
func modelFields(component string, t reflect.Type) (map[string]reflect.Type, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s: model must be a struct", component)
	}
	model := map[string]reflect.Type{}
	var collect func(reflect.Type) error
	collect = func(t reflect.Type) error {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous && f.Type == reflect.TypeFor[storyblok.Blok]() {
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if f.Anonymous && f.Type.Kind() == reflect.Struct && name == "" {
				if err := collect(f.Type); err != nil {
					return err
				}
				continue
			}
			if !f.IsExported() || name == "" {
				return fmt.Errorf("%s.%s: content fields require an explicit JSON name", component, f.Name)
			}
			if _, exists := model[name]; exists {
				return fmt.Errorf("%s: duplicate JSON field %s", component, name)
			}
			model[name] = f.Type
		}
		return nil
	}
	return model, collect(t)
}

func (d Definition[T]) validateField(f Field, model map[string]reflect.Type, seen map[string]bool) error {
	if !identifier.MatchString(f.Name) || seen[f.Name] {
		return fmt.Errorf("%s: invalid or duplicate schema field %q", d.Name, f.Name)
	}
	seen[f.Name] = true
	ft, ok := model[f.Name]
	if !ok {
		return fmt.Errorf("%s.%s: schema field missing from Go model", d.Name, f.Name)
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s.%s: "+format, append([]any{d.Name, f.Name}, args...)...)
	}
	if f.Type != "bloks" && (f.Allow != "" || len(f.Components) > 0 || f.Max != 0) {
		return fail("block restrictions only apply to bloks")
	}
	if f.Type != "option" && (len(f.Options) > 0 || f.Default != "") {
		return fail("options only apply to option fields")
	}
	if f.Type != "asset" && len(f.Filetypes) > 0 {
		return fail("filetypes only apply to asset fields")
	}
	switch f.Type {
	case "text", "textarea":
		if ft != reflect.TypeFor[string]() {
			return fail("%s requires a string", f.Type)
		}
	case "markdown":
		if ft != reflect.TypeFor[storyblok.Markdown]() {
			return fail("markdown requires a storyblok.Markdown")
		}
	case "option":
		if ft != reflect.TypeFor[string]() {
			return fail("option requires a string")
		}
		if len(f.Options) == 0 {
			return fail("option requires options")
		}
		if f.Default != "" && !slices.ContainsFunc(f.Options, func(o Option) bool { return o.Value == f.Default }) {
			return fail("default %q is not an option", f.Default)
		}
	case "asset":
		if ft != reflect.TypeFor[storyblok.Asset]() {
			return fail("asset requires a storyblok.Asset")
		}
	case "multilink":
		if ft != reflect.TypeFor[storyblok.Link]() {
			return fail("multilink requires a storyblok.Link")
		}
	case "bloks":
		meta := reflect.TypeFor[interface{ Meta() *storyblok.Blok }]()
		if ft.Kind() != reflect.Slice || !ft.Elem().Implements(meta) || f.Max < 0 {
			return fail("blocks require a slice of blocks and a nestable category")
		}
		if f.Allow != "" && len(f.Components) > 0 {
			return fail("restrict blocks to either a category or components")
		}
		if len(f.Components) == 0 && (!validCategory(f.Allow) || f.Allow == Page) {
			return fail("blocks require a slice of blocks and a nestable category")
		}
		if d.Category == Page && f.Allow != Section {
			return fail("pages only accept sections")
		}
	default:
		return fail("unsupported field type %q", f.Type)
	}
	return nil
}

// pageMetadataFields are the fields every page needs for its document title
// and meta description.
var pageMetadataFields = []Field{{Name: "title", Type: "text"}, {Name: "description", Type: "textarea"}}

func (d Definition[T]) validatePageMetadata() error {
	fields := d.flatFields()
	for _, want := range pageMetadataFields {
		i := slices.IndexFunc(fields, func(f Field) bool { return f.Name == want.Name })
		if i < 0 || fields[i].Type != want.Type || !fields[i].Required {
			return fmt.Errorf("%s: pages require a required %s field of type %s", d.Name, want.Name, want.Type)
		}
	}
	return nil
}

// flatFields lists content fields, including group members.
func (d Definition[T]) flatFields() []Field {
	var fields []Field
	for _, f := range d.Fields {
		if f.Type == groupType {
			fields = append(fields, f.Fields...)
			continue
		}
		fields = append(fields, f)
	}
	return fields
}

func validCategory(c Category) bool {
	return c == Page || c == Section || c == Media || c == Content || c == Site
}

// Compile resolves categories against the complete registry, never wildcards.
func (d Definition[T]) Compile(categories map[string]Category) (Component, error) {
	if err := d.Validate(); err != nil {
		return Component{}, err
	}
	root := d.Category == Page || d.ContentType
	c := Component{Name: d.Name, DisplayName: d.DisplayName, IsRoot: root, IsNestable: !root, Schema: map[string]map[string]any{}}
	pos := 0
	add := func(f Field) error {
		field, err := d.compileField(f, pos, categories)
		if err != nil {
			return err
		}
		c.Schema[f.Name] = field
		pos++
		return nil
	}
	for _, f := range d.Fields {
		if f.Type != groupType {
			if err := add(f); err != nil {
				return Component{}, err
			}
			continue
		}
		keys := make([]string, 0, len(f.Fields))
		for _, member := range f.Fields {
			keys = append(keys, member.Name)
		}
		c.Schema[f.Name] = map[string]any{"type": groupType, "pos": pos, "display_name": f.Label, "keys": keys}
		pos++
		for _, member := range f.Fields {
			if err := add(member); err != nil {
				return Component{}, err
			}
		}
	}
	return c, nil
}

func (d Definition[T]) compileField(f Field, pos int, categories map[string]Category) (map[string]any, error) {
	field := map[string]any{"type": f.Type, "pos": pos, "display_name": f.Label, "description": f.Description, "required": f.Required}
	switch f.Type {
	case "text", "textarea", "markdown":
		if !f.NotTranslatable {
			field["translatable"] = true
		}
	case "option":
		options := make([]map[string]any, 0, len(f.Options))
		for _, o := range f.Options {
			options = append(options, map[string]any{"name": o.Label, "value": o.Value})
		}
		field["options"] = options
		field["default_value"] = f.Default
		field["exclude_empty_option"] = true
	case "asset":
		if len(f.Filetypes) > 0 {
			field["filetypes"] = f.Filetypes
		}
	case "bloks":
		allowed := slices.Clone(f.Components)
		for _, name := range allowed {
			if _, ok := categories[name]; !ok {
				return nil, fmt.Errorf("%s.%s: component %s is not registered", d.Name, f.Name, name)
			}
		}
		if len(allowed) == 0 {
			for name, category := range categories {
				if category == f.Allow {
					allowed = append(allowed, name)
				}
			}
		}
		slices.Sort(allowed)
		if len(allowed) == 0 {
			return nil, fmt.Errorf("%s.%s: no registered %s components", d.Name, f.Name, f.Allow)
		}
		field["restrict_components"] = true
		field["component_whitelist"] = allowed
		if f.Max > 0 {
			field["maximum"] = f.Max
		}
	}
	return field, nil
}
