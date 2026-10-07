package server_test

import (
	"net/http"
	"strings"
	"testing"
)

const (
	germanLanding = `{"id": 20, "name": "Landing", "content": {
		"component": "page-landing-page", "_uid": "gl", "title": "Landing", "title__i18n__de": "Startseite",
		"description": "About", "description__i18n__de": "Über",
		"sections": [
			{"component": "block-section-intro", "_uid": "gl1", "heading": "Untranslated heading"},
			{"component": "block-section-content", "_uid": "gl2", "content": [
				{"component": "block-content-cta", "_uid": "gl3", "label": "Partners", "link": {"linktype": "story", "cached_url": "partners"}},
				{"component": "block-content-cta", "_uid": "gl4", "label": "Aktion", "link": {"linktype": "story", "cached_url": "de/aktion"}},
				{"component": "block-content-cta", "_uid": "gl5", "label": "Home", "link": {"linktype": "story", "cached_url": "home"}}
			]},
			{"component": "block-section-contact", "_uid": "contact", "heading": "Contact"}
		]
	}}`
	partnersStory = `{"id": 21, "name": "Partners", "alternates": [{"full_slug": "de/partner", "published": true}],
		"content": {"component": "page-landing-page", "_uid": "p", "title": "Partners", "description": "D"}}`
	germanPartnerStory = `{"id": 22, "name": "Partner", "alternates": [{"full_slug": "partners", "published": true}],
		"content": {"component": "page-landing-page", "_uid": "dp", "title": "Partner (folder)", "description": "D",
			"sections": [{"component": "block-section-content", "_uid": "dp1", "content": [
				{"component": "block-content-cta", "_uid": "dp2", "label": "Landing", "link": {"linktype": "story", "cached_url": "landing"}}]}]}}`
	germanOnlyStory = `{"id": 23, "name": "Aktion", "content": {"component": "page-landing-page", "_uid": "a", "title": "Aktion", "description": "D"}}`
)

func newI18nServer(t *testing.T) string {
	t.Helper()
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.stories["landing"] = germanLanding
	content.stories["partners"] = partnersStory
	content.stories["de/partner"] = germanPartnerStory
	content.stories["de/aktion"] = germanOnlyStory
	return ts.URL
}

func TestFieldLevelGermanFallsBackToDefaultFields(t *testing.T) {
	status, body := do(t, http.MethodGet, newI18nServer(t)+"/de/landing", "")
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	for _, want := range []string{`<html lang="de">`, "<title>Startseite</title>", "Untranslated heading", "Nachricht senden", `<link rel="canonical" href="https://example.com/de/landing">`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, "__i18n__") {
		t.Error("translation keys leaked")
	}
}

func TestGermanStoryLinksAreLocalized(t *testing.T) {
	base := newI18nServer(t)
	_, fieldLevel := do(t, http.MethodGet, base+"/de/landing", "")
	for _, want := range []string{`href="/de/partners">Partners</a>`, `href="/de/aktion">Aktion</a>`, `href="/de">Home</a>`} {
		if !strings.Contains(fieldLevel, want) {
			t.Errorf("field-level page lacks %s", want)
		}
	}
	_, folderLevel := do(t, http.MethodGet, base+"/de/partner", "")
	if !strings.Contains(folderLevel, `href="/de/landing">Landing</a>`) {
		t.Error("folder-level page links to the English page")
	}
}

func TestFolderLevelAlternateRedirects(t *testing.T) {
	res := request(t, http.MethodGet, newI18nServer(t)+"/de/partners", nil, nil)
	if res.status != http.StatusMovedPermanently || res.header.Get("Location") != "/de/partner" {
		t.Errorf("status %d, Location %q", res.status, res.header.Get("Location"))
	}
	// The alternate may be removed later; browsers must not keep the redirect.
	if res.header.Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %q", res.header.Get("Cache-Control"))
	}
}

func TestLanguageAlternates(t *testing.T) {
	base := newI18nServer(t)
	_, folder := do(t, http.MethodGet, base+"/de/partner", "")
	for _, want := range []string{
		`<link rel="alternate" hreflang="en" href="https://example.com/partners">`,
		`<link rel="alternate" hreflang="de" href="https://example.com/de/partner">`,
		`<link rel="alternate" hreflang="x-default" href="https://example.com/partners">`,
		`<meta property="og:locale" content="de_DE">`, `<meta property="og:locale:alternate" content="en_US">`,
		`href="/partners" hreflang="en" lang="en">English</a>`,
		`href="/de/partner" hreflang="de" lang="de" aria-current="true">Deutsch</a>`,
	} {
		if !strings.Contains(folder, want) {
			t.Errorf("folder-level page lacks %s", want)
		}
	}
	_, germanOnly := do(t, http.MethodGet, base+"/de/aktion", "")
	if strings.Contains(germanOnly, `hreflang="en" href=`) {
		t.Error("German-only page claims an English version")
	}
	if !strings.Contains(germanOnly, `href="/" hreflang="en" lang="en">English</a>`) {
		t.Error("language switcher of a German-only page should link to the English home page")
	}
}

func TestGermanUIStringsAndSettings(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.stories["settings"] = strings.Replace(settingsStory, `"site_name": "Acme"`, `"site_name": "Acme", "copyright__i18n__de": "© 2026 Acme GmbH"`, 1)
	_, body := do(t, http.MethodGet, ts.URL+"/de", "")
	for _, want := range []string{"Zum Inhalt springen", ">Menü</button>", "© 2026 Acme GmbH", `<a class="site-header__home" href="/de">`, `href="/de/landing"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	status, notFound := do(t, http.MethodGet, ts.URL+"/de/missing", "")
	if status != http.StatusNotFound || !strings.Contains(notFound, "Seite nicht gefunden") {
		t.Errorf("German 404: status %d", status)
	}
}

func TestDoubleLocalePrefixIsNotFound(t *testing.T) {
	if status, _ := do(t, http.MethodGet, newI18nServer(t)+"/de/de/aktion", ""); status != http.StatusNotFound {
		t.Errorf("status %d, want 404", status)
	}
}

func TestPreviewSelectsLanguageFromTheEditor(t *testing.T) {
	_, body := do(t, http.MethodGet, newI18nServer(t)+"/landing"+previewQuery()+"&_storyblok_lang=de", "")
	if !strings.Contains(body, "<title>Startseite</title>") || !strings.Contains(body, `<html lang="de">`) {
		t.Error("preview ignores the editor's language")
	}
}

func TestSitemapListsEveryLanguageVersion(t *testing.T) {
	_, body := do(t, http.MethodGet, newI18nServer(t)+"/sitemap.xml", "")
	for _, want := range []string{
		`xmlns:xhtml="http://www.w3.org/1999/xhtml"`,
		"<loc>https://example.com/landing</loc>", "<loc>https://example.com/de/landing</loc>",
		"<loc>https://example.com/partners</loc>", "<loc>https://example.com/de/partner</loc>",
		"<loc>https://example.com/de/aktion</loc>",
		`<xhtml:link rel="alternate" hreflang="de" href="https://example.com/de/partner"></xhtml:link>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap lacks %s", want)
		}
	}
	if strings.Contains(body, "https://example.com/de/partners<") {
		t.Error("sitemap lists the field-level URL of a page with a folder-level translation")
	}
}

func TestFieldLevelGermanLinksItsEnglishVersion(t *testing.T) {
	_, body := do(t, http.MethodGet, newI18nServer(t)+"/de/landing", "")
	for _, want := range []string{
		`<link rel="alternate" hreflang="en" href="https://example.com/landing">`,
		`<link rel="alternate" hreflang="de" href="https://example.com/de/landing">`,
		`href="/landing" hreflang="en" lang="en">English</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestGermanListingsOfGermanFolders(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.stories["de/blog-start"] = `{"id": 30, "name": "Blog", "content": {"component": "page-landing-page", "_uid": "bs",
		"title": "Blog", "description": "D", "sections": [
			{"component": "block-section-articles", "_uid": "ba", "heading": "Neu", "folder": "de/blog"}]}}`
	_, body := do(t, http.MethodGet, ts.URL+"/de/blog-start", "")
	if !strings.Contains(body, `href="/de/blog/article-14"`) || strings.Contains(body, "/de/de/") {
		t.Errorf("article links:\n%s", articleLinks(body))
	}
}

func articleLinks(body string) string {
	var links []string
	for _, part := range strings.Split(body, `class="base-card__link" href="`)[1:] {
		links = append(links, part[:strings.Index(part, `"`)])
	}
	return strings.Join(links, "\n")
}
