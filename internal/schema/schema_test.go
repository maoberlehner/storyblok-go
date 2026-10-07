package schema

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"storyblok-go-website/internal/storyblok"
)

type testBlock interface{ Meta() *storyblok.Blok }
type testPage struct {
	storyblok.Blok
	Title       string      `json:"title"`
	Sections    []testBlock `json:"sections"`
	Description string      `json:"description"`
}

func validDefinition() Definition[testPage] {
	return Definition[testPage]{Name: "page-test", Category: Page, Fields: []Field{
		{Name: "title", Type: "text", Required: true}, {Name: "sections", Type: "bloks", Allow: Section},
		{Name: "description", Type: "textarea", Required: true},
	}}
}

func TestModelValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*Definition[testPage])
		want   string
	}{
		{"missing schema", func(d *Definition[testPage]) { d.Fields = slices.Delete(d.Fields, 1, 2) }, "has no schema"},
		{"missing Go field", func(d *Definition[testPage]) { d.Fields[1].Name = "missing" }, "missing from Go"},
		{"wrong type", func(d *Definition[testPage]) { d.Fields[1].Type = "text"; d.Fields[1].Allow = "" }, "requires a string"},
		{"duplicate", func(d *Definition[testPage]) { d.Fields = append(d.Fields, d.Fields[0]) }, "duplicate"},
		{"wrong children", func(d *Definition[testPage]) { d.Fields[1].Allow = Content }, "only accept sections"},
		{"no restrictions", func(d *Definition[testPage]) { d.Fields[1].Allow = "" }, "nestable category"},
		{"base", func(d *Definition[testPage]) { d.Name = "base-test" }, "invalid component"},
		{"page without description", func(d *Definition[testPage]) { d.Fields = d.Fields[:2] }, "pages require a required description"},
		{"optional page title", func(d *Definition[testPage]) { d.Fields[0].Required = false }, "pages require a required title"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := validDefinition()
			tt.mutate(&d)
			err := d.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v; want %s", err, tt.want)
			}
		})
	}
}

func TestCompileResolvesOnlySectionsInStableOrder(t *testing.T) {
	d := validDefinition()
	c, err := d.Compile(map[string]Category{"block-section-z": Section, "block-content-a": Content, "block-section-a": Section, "block-media-image": Media})
	if err != nil {
		t.Fatal(err)
	}
	field := c.Schema["sections"]
	if !reflect.DeepEqual(field["component_whitelist"], []string{"block-section-a", "block-section-z"}) || field["restrict_components"] != true {
		t.Fatalf("bad restriction: %#v", field)
	}
	if !c.IsRoot || c.IsNestable || field["pos"] != 1 {
		t.Fatalf("bad page metadata: %#v", c)
	}
	if _, err := d.Compile(nil); err == nil {
		t.Fatal("empty category silently allowed")
	}
}

type testLinkBlock struct {
	storyblok.Blok
	Link  storyblok.Link `json:"link"`
	Label string         `json:"label"`
}

func TestMultilinkFields(t *testing.T) {
	d := Definition[testLinkBlock]{Name: "block-content-test", Category: Content, Fields: []Field{
		{Name: "link", Type: "multilink"}, {Name: "label", Type: "text"},
	}}
	c, err := d.Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Schema["link"]["type"] != "multilink" {
		t.Fatalf("bad link field: %#v", c.Schema["link"])
	}

	d.Fields[1].Type = "multilink"
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "requires a storyblok.Link") {
		t.Fatalf("got %v; want type mismatch", err)
	}
}

type testRichBlock struct {
	storyblok.Blok
	testStyle
	Body  storyblok.Markdown `json:"body"`
	Image storyblok.Asset    `json:"image"`
	Size  string             `json:"size"`
	Media []testBlock        `json:"media"`
	Note  string             `json:"note"`
}

// testStyle is embedded like the section palette, so its fields are content fields.
type testStyle struct {
	Background string `json:"background"`
}

func richDefinition() Definition[testRichBlock] {
	return Definition[testRichBlock]{Name: "block-section-rich", Category: Section, Fields: []Field{
		{Name: "body", Type: "markdown", Label: "Body"},
		{Name: "image", Type: "asset", Label: "Image", Filetypes: []string{"images"}},
		{Name: "size", Type: "option", Label: "Size", Options: []Option{{Value: "small", Label: "Small"}, {Value: "large", Label: "Large"}}, Default: "small"},
		{Name: "media", Type: "bloks", Label: "Media", Allow: Media, Max: 1},
		{Name: "style", Type: "section", Label: "Style", Fields: []Field{
			{Name: "background", Type: "text", Label: "Background", NotTranslatable: true},
			{Name: "note", Type: "text", Label: "Note"},
		}},
	}}
}

func TestRichFieldsCompile(t *testing.T) {
	c, err := richDefinition().Compile(map[string]Category{"block-media-image": Media})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Schema["body"]; got["type"] != "markdown" || got["translatable"] != true {
		t.Errorf("markdown field: %#v", got)
	}
	if got := c.Schema["image"]; got["type"] != "asset" || !reflect.DeepEqual(got["filetypes"], []string{"images"}) {
		t.Errorf("asset field: %#v", got)
	}
	size := c.Schema["size"]
	wantOptions := []map[string]any{{"name": "Small", "value": "small"}, {"name": "Large", "value": "large"}}
	if size["type"] != "option" || !reflect.DeepEqual(size["options"], wantOptions) || size["default_value"] != "small" || size["exclude_empty_option"] != true {
		t.Errorf("option field: %#v", size)
	}
	if got := c.Schema["media"]; got["maximum"] != 1 || !reflect.DeepEqual(got["component_whitelist"], []string{"block-media-image"}) {
		t.Errorf("media field: %#v", got)
	}
	style := c.Schema["style"]
	if style["type"] != "section" || !reflect.DeepEqual(style["keys"], []string{"background", "note"}) {
		t.Errorf("group: %#v", style)
	}
	if _, ok := c.Schema["background"]["translatable"]; ok {
		t.Errorf("opted-out field is translatable: %#v", c.Schema["background"])
	}
	if c.Schema["note"]["translatable"] != true {
		t.Errorf("grouped text field is not translatable: %#v", c.Schema["note"])
	}
}

func TestRichFieldValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*Definition[testRichBlock])
		want   string
	}{
		{"markdown as string", func(d *Definition[testRichBlock]) {
			d.Fields[0].Name = "note"
			d.Fields[4].Fields = d.Fields[4].Fields[:1]
		}, "requires a storyblok.Markdown"},
		{"asset type", func(d *Definition[testRichBlock]) { d.Fields[1].Type = "markdown"; d.Fields[1].Filetypes = nil }, "requires a storyblok.Markdown"},
		{"option without options", func(d *Definition[testRichBlock]) { d.Fields[2].Options = nil }, "requires options"},
		{"unknown default", func(d *Definition[testRichBlock]) { d.Fields[2].Default = "huge" }, "default"},
		{"nested group", func(d *Definition[testRichBlock]) {
			d.Fields[4].Fields = append(d.Fields[4].Fields, Field{Name: "inner", Type: "section", Fields: []Field{{Name: "x", Type: "text"}}})
		}, "cannot be nested"},
		{"empty group", func(d *Definition[testRichBlock]) {
			d.Fields = append(d.Fields, Field{Name: "empty", Type: "section"})
		}, "empty group"},
		{"group named like a field", func(d *Definition[testRichBlock]) { d.Fields[4].Name = "note" }, "duplicate"},
		{"embedded field without schema", func(d *Definition[testRichBlock]) { d.Fields[4].Fields = d.Fields[4].Fields[1:] }, "background: Go content field has no schema"},
		{"allow and components", func(d *Definition[testRichBlock]) { d.Fields[3].Components = []string{"block-media-image"} }, "either a category or components"},
		{"restriction on text", func(d *Definition[testRichBlock]) { d.Fields[4].Fields[1].Max = 2 }, "only apply to bloks"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := richDefinition()
			tt.mutate(&d)
			if err := d.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v; want %s", err, tt.want)
			}
		})
	}
}

type testSettings struct {
	storyblok.Blok
	Links []testBlock `json:"links"`
}

func TestSiteContentTypes(t *testing.T) {
	d := Definition[testSettings]{Name: "site-settings", Category: Site, ContentType: true, Fields: []Field{
		{Name: "links", Type: "bloks", Components: []string{"site-link"}},
	}}
	c, err := d.Compile(map[string]Category{"site-link": Site, "site-settings": Site})
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsRoot || c.IsNestable || !reflect.DeepEqual(c.Schema["links"]["component_whitelist"], []string{"site-link"}) {
		t.Fatalf("bad site content type: %#v", c)
	}
	if _, err := d.Compile(map[string]Category{"site-settings": Site}); err == nil || !strings.Contains(err.Error(), "site-link") {
		t.Fatalf("unregistered component allowed: %v", err)
	}
	section := Definition[testSettings]{Name: "block-section-x", Category: Section, ContentType: true, Fields: d.Fields}
	if err := section.Validate(); err == nil || !strings.Contains(err.Error(), "content types") {
		t.Fatalf("section accepted as content type: %v", err)
	}
}

func TestTextFieldsAreTranslatable(t *testing.T) {
	c, err := validDefinition().Compile(map[string]Category{"block-section-a": Section})
	if err != nil {
		t.Fatal(err)
	}
	if c.Schema["title"]["translatable"] != true || c.Schema["description"]["translatable"] != true {
		t.Fatalf("text fields not translatable: %#v", c.Schema)
	}
	if _, ok := c.Schema["sections"]["translatable"]; ok {
		t.Fatal("bloks field marked translatable")
	}
}
