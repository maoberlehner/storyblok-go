package components

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

func contentSection(blocks string) string {
	return `{"component":"page-landing-page","title":"T","description":"D","sections":[
		{"component":"block-section-content","content":[` + blocks + `]}]}`
}

func TestMarkdownText(t *testing.T) {
	out := renderStory(t, contentSection(`{"component":"block-content-text","text":"# Big\n\nSome **bold** text.\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n| a | b |\n|---|---|\n| 1 | 2 |"}`))
	for _, want := range []string{"<h2>Big</h2>", "<strong>bold</strong>", "<table>", "<td>1</td>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, unwanted := range []string{"<h1>Big", "<script>alert", `href="javascript:`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("rendered %s", unwanted)
		}
	}
}

func TestHeadlineLevels(t *testing.T) {
	out := renderStory(t, contentSection(`
		{"component":"block-content-headline","text":"Default"},
		{"component":"block-content-headline","text":"Three","level":"h3"},
		{"component":"block-content-headline","text":"Four","level":"h4"},
		{"component":"block-content-headline","text":"Bogus","level":"h1"}`))
	for _, want := range []string{
		`class="block-content-headline">Default</h2>`, `class="block-content-headline">Three</h3>`,
		`class="block-content-headline">Four</h4>`, `class="block-content-headline">Bogus</h2>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestCallToActionVariants(t *testing.T) {
	out := renderStory(t, contentSection(`
		{"component":"block-content-cta","label":"Buy","link":{"linktype":"url","url":"https://example.com/buy"}},
		{"component":"block-content-cta","label":"Learn","variant":"secondary","link":{"linktype":"story","cached_url":"docs"}},
		{"component":"block-content-cta","label":"Nowhere"}`))
	for _, want := range []string{
		`<a class="base-button" href="https://example.com/buy">Buy</a>`,
		`<a class="base-button base-button--secondary" href="/docs">Learn</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "Nowhere") {
		t.Error("rendered a CTA without a link")
	}
}

func TestContentSectionLayout(t *testing.T) {
	image := `{"component":"block-media-image","image":{"filename":"` + photo + `"}}`
	out := renderStory(t, `{"component":"page-landing-page","title":"T","description":"D","sections":[
		{"component":"block-section-content","content":[{"component":"block-content-headline","text":"Only text"}]},
		{"component":"block-section-content","media_position":"start","background":"muted","media":[`+image+`],
		 "content":[{"component":"block-content-text","text":"With media"}]}]}`)
	for _, want := range []string{
		`class="block-section-content__layout"`,
		`class="block-section-content__layout block-section-content__layout--media block-section-content__layout--media-start"`,
		`class="block-section-content base-section base-section--muted"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, `fetchpriority="high"`) {
		t.Error("image outside the first section is prioritized")
	}
}

type manyArticles struct{}

func (manyArticles) Stories(context.Context, storyblok.StoriesOptions) (storyblok.StoryList, error) {
	return storyblok.StoryList{Stories: []byte(`[{"full_slug":"articles/a","content":{"title":"A"}}]`), Total: 1500}, nil
}

func TestArticleCountUsesTheLocalesNumberFormat(t *testing.T) {
	var story storyblok.Story[AnyBlock]
	if err := json.Unmarshal([]byte(`{"name":"P","content":{"component":"page-landing-page","title":"T","description":"D",
		"sections":[{"component":"block-section-articles","_uid":"a1","heading":"H","folder":"articles"}]}}`), &story); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSections(t.Context(), manyArticles{}, story.Content.Block, Request{Locale: locale.German})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	page := NewPage(story)
	page.Locale = locale.German
	page.Loaded = loaded
	var out bytes.Buffer
	if err := r.Page(&out, page); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "6 von 1.500 Artikeln") {
		t.Error("count not formatted for German")
	}
}
