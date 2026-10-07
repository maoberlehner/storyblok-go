package components

import (
	"bytes"
	"encoding/json/v2"
	"strings"
	"testing"

	"storyblok-go-website/internal/storyblok"
)

func renderPage(t *testing.T, content string, chrome *Chrome) string {
	t.Helper()
	var story storyblok.Story[AnyBlock]
	if err := json.Unmarshal([]byte(`{"name":"P","content":`+content+`}`), &story); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	page := NewPage(story)
	page.Chrome = chrome
	page.Canonical = "https://example.com/p"
	var out bytes.Buffer
	if err := r.Page(&out, page); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func assertContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestMetaFallsBackToTitleAndDescription(t *testing.T) {
	out := renderPage(t, `{"component":"page-landing-page","title":"Title","description":"Desc"}`, nil)
	assertContains(t, out,
		"<title>Title</title>", `<meta name="description" content="Desc">`,
		`<meta property="og:title" content="Title">`, `<meta property="og:description" content="Desc">`,
		`<meta property="og:type" content="website">`, `<meta property="og:url" content="https://example.com/p">`,
		`<meta name="twitter:card" content="summary">`)
	if strings.Contains(out, "og:image") {
		t.Error("og:image without any image")
	}
}

func TestSearchOverridesFeedSharing(t *testing.T) {
	out := renderPage(t, `{"component":"page-landing-page","title":"Title","description":"Desc",
		"seo_title":"Search title","seo_description":"Search desc","og_description":"Share desc"}`, nil)
	assertContains(t, out,
		"<title>Search title</title>", `<meta name="description" content="Search desc">`,
		`<meta property="og:title" content="Search title">`, `<meta property="og:description" content="Share desc">`)
}

func TestShareImage(t *testing.T) {
	settings := &SiteSettings{SiteName: "Acme", DefaultOGImage: storyblok.Asset{Filename: "https://a.storyblok.com/f/1/2400x1600/x/default.jpg", Alt: "Default"}}
	chrome := &Chrome{Settings: settings, HomeHref: "/"}

	withDefault := renderPage(t, `{"component":"page-landing-page","title":"T","description":"D"}`, chrome)
	assertContains(t, withDefault,
		`<meta property="og:image" content="https://a.storyblok.com/f/1/2400x1600/x/default.jpg/m/1200x630">`,
		`<meta property="og:image:width" content="1200">`, `<meta property="og:image:height" content="630">`,
		`<meta property="og:image:alt" content="Default">`, `<meta property="og:site_name" content="Acme">`,
		`<meta name="twitter:card" content="summary_large_image">`)

	own := renderPage(t, `{"component":"page-article","title":"T","description":"D",
		"og_image":{"filename":"https://a.storyblok.com/f/1/2000x1000/y/own.jpg","focus":"1x2:3x4"}}`, chrome)
	assertContains(t, own,
		`content="https://a.storyblok.com/f/1/2000x1000/y/own.jpg/m/1200x630/filters:focal(1x2:3x4)"`,
		`<meta property="og:type" content="article">`)

	svg := renderPage(t, `{"component":"page-landing-page","title":"T","description":"D",
		"og_image":{"filename":"https://a.storyblok.com/f/1/10x10/z/logo.svg"}}`, nil)
	if strings.Contains(svg, "og:image") {
		t.Error("SVG used as share image, which link previews don't support")
	}
}

func TestFAQStructuredData(t *testing.T) {
	out := renderPage(t, `{"component":"page-landing-page","title":"T","description":"D","sections":[
		{"component":"block-section-faq","heading":"FAQ","items":[
			{"component":"block-content-question","question":"Why?","answer":"Because </script><script>alert(1)</script>"},
			{"component":"block-content-question","question":"Unanswered"}]}]}`, nil)
	start := strings.Index(out, `<script type="application/ld+json">`)
	if start < 0 {
		t.Fatal("no structured data")
	}
	end := strings.Index(out[start:], "</script>")
	data := out[start+len(`<script type="application/ld+json">`) : start+end]
	var faq struct {
		Type       string `json:"@type"`
		MainEntity []struct {
			Name   string `json:"name"`
			Answer struct {
				Text string `json:"text"`
			} `json:"acceptedAnswer"`
		} `json:"mainEntity"`
	}
	if err := json.Unmarshal([]byte(data), &faq); err != nil {
		t.Fatalf("invalid JSON-LD %q: %v", data, err)
	}
	if faq.Type != "FAQPage" || len(faq.MainEntity) != 1 || faq.MainEntity[0].Name != "Why?" ||
		faq.MainEntity[0].Answer.Text != "Because </script><script>alert(1)</script>" {
		t.Errorf("structured data = %+v", faq)
	}
}
