package components

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"storyblok-go-website/internal/storyblok"
)

func TestEveryCMSComponentHasCompanionFiles(t *testing.T) {
	schemas, err := Schemas()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range schemas {
		for _, suffix := range []string{".go", ".schema.go", ".html", ".css"} {
			if _, err := os.Stat(s.Name + suffix); err != nil {
				t.Errorf("missing companion: %v", err)
			}
		}
	}
}

func TestLandingPageRendersSectionsAndEscapesContent(t *testing.T) {
	var story storyblok.Story[AnyBlock]
	err := json.Unmarshal([]byte(`{"name":"Fallback","content":{"component":"page-landing-page","title":"Hello <world>","description":"About <world>","sections":[{"component":"block-section-intro","heading":"Welcome","text":"<script>alert(1)</script>"}]}}`), &story)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := r.Page(&out, NewPage(story)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<title>Hello &lt;world&gt;</title>",
		`<meta name="description" content="About &lt;world&gt;">`,
		`<h1 class="page-landing-page__title">Hello &lt;world&gt;</h1>`,
		`class="block-section-intro"`, "<h2", "&lt;script&gt;alert(1)&lt;/script&gt;",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out.String(), "<script>alert") {
		t.Fatal("unescaped content")
	}
}

func TestPageRejectsNestedPage(t *testing.T) {
	var block AnyBlock
	if err := json.Unmarshal([]byte(`{"component":"page-landing-page","sections":[{"component":"page-landing-page","title":"Nested"}]}`), &block); err != nil {
		t.Fatal(err)
	}
	if unknown, ok := block.Block.(*Unknown); !ok || unknown.Err == nil {
		t.Fatalf("accepted nested page: %#v", block)
	}
}

func TestBlocksAreIdentifiedByShortIDs(t *testing.T) {
	var block AnyBlock
	if err := json.Unmarshal([]byte(`{"component":"block-section-articles","_uid":"6f1c2a8e-0b1d-4c55-9a7e-1d2f3a4b5c04"}`), &block); err != nil {
		t.Fatal(err)
	}
	articles := block.Block.(*BlockSectionArticles)
	if got := ElementID(articles); got != "b-6f1c2a8e" {
		t.Errorf("ElementID = %q", got)
	}
	if got := articles.StateParams(); len(got) != 1 || got[0] != "page-6f1c2a8e" {
		t.Errorf("StateParams = %q", got)
	}
	page := &PageLandingPage{Sections: Blocks{articles}}
	if found, ok := FindSection(page, "6f1c2a8e"); !ok || found != Block(articles) {
		t.Error("section not found by short ID")
	}
}
