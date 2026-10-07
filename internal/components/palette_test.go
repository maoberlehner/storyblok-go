package components

import (
	"strings"
	"testing"
)

func TestSectionBackgrounds(t *testing.T) {
	out := renderStory(t, `{"component":"page-landing-page","title":"T","description":"D","sections":[
		{"component":"block-section-intro","heading":"Plain"},
		{"component":"block-section-intro","heading":"Muted","background":"muted"},
		{"component":"block-section-cta","heading":"Call","link":{"linktype":"url","url":"https://example.com"},"link_label":"Go"},
		{"component":"block-section-cta","heading":"Calm","background":"default","link":{"linktype":"url","url":"https://example.com"},"link_label":"Go"},
		{"component":"block-section-intro","heading":"Bogus","background":"neon"}]}`)
	for _, want := range []string{
		`class="block-section-intro base-section"`,
		`class="block-section-intro base-section base-section--muted"`,
		`class="block-section-cta base-section base-section--accent"`,
		`class="block-section-cta base-section"`,
		".base-section--accent{", // the palette's styles are inlined once used
	} {
		if !strings.Contains(strings.ReplaceAll(out, " {", "{"), want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "neon") {
		t.Error("unknown background reached the markup")
	}
}

func TestEverySectionOffersTheBackgroundField(t *testing.T) {
	schemas, err := Schemas()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range schemas {
		if !strings.HasPrefix(s.Name, "block-section-") {
			continue
		}
		field, ok := s.Schema["background"]
		if !ok || field["type"] != "option" {
			t.Errorf("%s has no background option", s.Name)
		}
	}
}
