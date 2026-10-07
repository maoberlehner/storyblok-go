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
