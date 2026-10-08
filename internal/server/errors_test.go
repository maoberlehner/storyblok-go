package server_test

import (
	"net/http"
	"strings"
	"testing"
)

const notFoundStory = `{"id": 4, "name": "Not found", "content": {
	"component": "page-landing-page", "_uid": "nf", "title": "Lost?", "description": "Nothing here",
	"sections": [{"component": "block-section-intro", "_uid": "nf1", "heading": "Try the home page"}]
}}`

func TestNotFoundPageFromCMS(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.stories["error-404"] = notFoundStory
	status, body := do(t, http.MethodGet, ts.URL+"/missing", "")
	if status != http.StatusNotFound {
		t.Fatalf("status %d", status)
	}
	for _, want := range []string{"<title>Lost?</title>", "Try the home page", `class="site-header__home"`, `<meta name="robots" content="noindex">`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, `rel="canonical"`) {
		t.Error("404 page has a canonical URL")
	}
	if status, _ := do(t, http.MethodGet, ts.URL+"/error-404", ""); status != http.StatusNotFound {
		t.Errorf("/error-404: status %d, want 404", status)
	}
}

func TestNotFoundPageLoadsItsSections(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.stories["error-404"] = `{"id": 4, "name": "Not found", "content": {
		"component": "page-landing-page", "_uid": "nf", "title": "Lost?",
		"sections": [{"component": "block-section-articles", "_uid": "arts", "heading": "Latest", "folder": "articles"}]
	}}`
	status, body := do(t, http.MethodGet, ts.URL+"/missing", "")
	if status != http.StatusNotFound || !strings.Contains(body, ">Article 14</a>") {
		t.Errorf("status %d, body:\n%s", status, body)
	}
}

func TestBuiltInNotFoundPage(t *testing.T) {
	status, body := do(t, http.MethodGet, newServer(t).URL+"/missing", "")
	if status != http.StatusNotFound || !strings.Contains(body, "Page not found") || !strings.Contains(body, `class="site-header__home"`) {
		t.Errorf("status %d, body:\n%s", status, body)
	}
}

func TestServerErrorPageUsesLastKnownSettings(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	if status, _ := do(t, http.MethodGet, ts.URL+"/", ""); status != http.StatusOK {
		t.Fatal("home page failed")
	}
	content.stories["settings"] = rateLimited
	status, body := do(t, http.MethodGet, ts.URL+"/busy", "")
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "Something went wrong") || !strings.Contains(body, ">Acme</a>") {
		t.Errorf("status %d, body:\n%s", status, body)
	}
}

func TestServerErrorPageWithoutSettings(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	delete(content.stories, "settings")
	status, body := do(t, http.MethodGet, ts.URL+"/busy", "")
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "Something went wrong") || strings.Contains(body, "site-header") {
		t.Errorf("status %d, body:\n%s", status, body)
	}
}

func TestEnhancedRequestsGetPlainErrors(t *testing.T) {
	res := request(t, http.MethodGet, newServer(t).URL+"/busy", nil, map[string]string{"HX-Request": "true"})
	if res.status != http.StatusServiceUnavailable || strings.Contains(res.body, "<html") {
		t.Errorf("status %d, body %q", res.status, res.body)
	}
}

// htmx swaps a history restore response into the body, so it must be a page.
func TestHistoryRestoresGetErrorPages(t *testing.T) {
	ts := newServer(t)
	for path, status := range map[string]int{"/busy": http.StatusServiceUnavailable, "/missing": http.StatusNotFound} {
		res := request(t, http.MethodGet, ts.URL+path, nil, historyRestore)
		if res.status != status || !strings.Contains(res.body, "<html") {
			t.Errorf("%s: status %d, body %q", path, res.status, res.body)
		}
	}
}

func TestSitemapSkipsReservedStories(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.stories["error-404"] = notFoundStory
	_, body := do(t, http.MethodGet, ts.URL+"/sitemap.xml", "")
	if strings.Contains(body, "error-404") || strings.Contains(body, "/settings") {
		t.Errorf("sitemap lists reserved stories:\n%s", body)
	}
}
