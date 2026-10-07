package server_test

import (
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"storyblok-go-website/internal/components"
)

type response struct {
	status int
	header http.Header
	body   string
}

func request(t *testing.T, method, target string, form url.Values, header map[string]string) response {
	t.Helper()
	if form != nil && method == http.MethodPost && !form.Has(components.StartedField) {
		form = maps.Clone(form)
		form.Set(components.StartedField, validFormToken())
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(t.Context(), method, target, body)
	if err != nil {
		t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{res.StatusCode, res.Header, string(b)}
}

var enhanced = map[string]string{"HX-Request": "true"}

var articleTitle = regexp.MustCompile(`>Article (\d+)</a>`)

func articleTitles(body string) []string {
	var titles []string
	for _, m := range articleTitle.FindAllStringSubmatch(body, -1) {
		titles = append(titles, m[1])
	}
	return titles
}

// sectionHTML returns the markup of the section with the given element ID.
func sectionHTML(t *testing.T, body, id string) string {
	t.Helper()
	_, section, ok := strings.Cut(body, `id="`+id+`"`)
	if !ok {
		t.Fatalf("no section %s", id)
	}
	section, _, _ = strings.Cut(section, "</section>")
	return section
}

func TestLoadMore(t *testing.T) {
	ts := newServer(t)

	t.Run("shows the first page with a form for the next one", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing", nil, nil)
		section := sectionHTML(t, res.body, "b-arts")
		if got := strings.Join(articleTitles(section), ","); got != "14,13,12,11,10,9" {
			t.Errorf("articles = %s", got)
		}
		for _, want := range []string{
			"Showing 6 of 14 articles",
			`action="/landing"`,
			`name="page-arts" value="2"`,
			`hx-vals='{"_block": "arts"}'`,
		} {
			if !strings.Contains(section, want) {
				t.Errorf("section does not contain %q", want)
			}
		}
		if strings.Contains(res.body, "autofocus") {
			t.Error("first page moves focus")
		}
	})

	t.Run("renders all loaded pages without JavaScript and focuses the first new article", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?page-arts=2", nil, nil)
		section := sectionHTML(t, res.body, "b-arts")
		if got := len(articleTitles(section)); got != 12 {
			t.Errorf("got %d articles, want 12", got)
		}
		if !strings.Contains(section, `id="b-arts-item-7"`) || !strings.Contains(section, `href="/articles/article-8" autofocus>`) {
			t.Error("first new article is not anchored and focused")
		}
		if !strings.Contains(section, `name="page-arts" value="3"`) {
			t.Error("missing form for the third page")
		}
	})

	t.Run("keeps the state of every listing without JavaScript", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?page-arts=2&page-more=3", nil, nil)
		if got := len(articleTitles(sectionHTML(t, res.body, "b-arts"))); got != 12 {
			t.Errorf("first listing has %d articles, want 12", got)
		}
		other := sectionHTML(t, res.body, "b-more")
		if got := len(articleTitles(other)); got != 14 {
			t.Errorf("second listing has %d articles, want 14", got)
		}
		first := sectionHTML(t, res.body, "b-arts")
		if !strings.Contains(first, `<input type="hidden" name="page-more" value="3">`) {
			t.Error("first listing's form drops the second listing's state")
		}
	})

	t.Run("returns only the requested page as a fragment to htmx", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?page-arts=2&_block=arts", nil, enhanced)
		if got := strings.Join(articleTitles(res.body), ","); got != "8,7,6,5,4,3" {
			t.Errorf("articles = %s", got)
		}
		if strings.Contains(res.body, "<html") || !strings.Contains(res.body, `<hx-partial hx-target="#b-arts-more" hx-swap="outerHTML">`) {
			t.Errorf("body is not the fragment:\n%s", res.body)
		}
		if !strings.Contains(strings.Join(res.header.Values("Vary"), ","), "HX-Request") {
			t.Error("response does not vary by HX-Request")
		}
	})

	t.Run("updates only the requested listing in the browser URL", func(t *testing.T) {
		// The form still carries page-more=1 from its first render.
		header := map[string]string{"HX-Request": "true", "HX-Current-URL": ts.URL + "/landing?page-more=3&page-arts=1&sent=contact"}
		res := request(t, http.MethodGet, ts.URL+"/landing?page-more=1&page-arts=2&_block=arts", nil, header)
		if got := res.header.Get("HX-Replace-Url"); got != "/landing?page-arts=2&page-more=3" {
			t.Errorf("HX-Replace-Url = %q", got)
		}
	})

	t.Run("removes the form after the last page", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?page-arts=3&_block=arts", nil, enhanced)
		if got := strings.Join(articleTitles(res.body), ","); got != "2,1" {
			t.Errorf("articles = %s", got)
		}
		if !strings.Contains(res.body, "Showing 14 of 14 articles") || strings.Contains(res.body, "<form") {
			t.Errorf("unexpected more region:\n%s", res.body)
		}
	})
}

func validContact() url.Values {
	return url.Values{
		components.TargetParam: {"contact"},
		"name":                 {"Ada"},
		"email":                {"ada@example.com"},
		"topic":                {"sales"},
		"message":              {"Hello\r\nthere"},
		"consent":              {"yes"},
	}
}

func TestContactForm(t *testing.T) {
	inbox := &recordingInbox{}
	ts := newServerWithInbox(t, inbox)
	invalid := url.Values{components.TargetParam: {"contact"}, "name": {"Ada"}, "email": {"not-an-email"}}

	t.Run("re-renders the page with all errors without JavaScript", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing", invalid, nil)
		if res.status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d", res.status)
		}
		for _, want := range []string{
			"<title>Error: Landing</title>",
			`class="base-form-error-summary" id="b-contact-form-errors" tabindex="-1" autofocus`,
			`<a href="#b-contact-email">Enter an email address in the correct format, like name@example.com</a>`,
			`<a href="#b-contact-topic">Choose a topic</a>`,
			`<a href="#b-contact-message">Enter your message</a>`,
			`<a href="#b-contact-consent">Agree to the privacy policy to send your message</a>`,
			`value="not-an-email" autocomplete="email" spellcheck="false" required aria-describedby="b-contact-email-hint b-contact-email-error" aria-invalid="true"`,
			`value="Ada" autocomplete="name" required>`,
			// The rest of the page still renders.
			"Showing 6 of 14 articles",
		} {
			if !strings.Contains(res.body, want) {
				t.Errorf("body does not contain %q", want)
			}
		}
		if strings.Contains(res.body, `href="#b-contact-name"`) {
			t.Error("valid field is listed as an error")
		}
		if len(inbox.submissions) != 0 {
			t.Error("invalid form was delivered")
		}
	})

	t.Run("keeps listing state when the form re-renders without JavaScript", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing?page-arts=2", invalid, nil)
		if got := len(articleTitles(sectionHTML(t, res.body, "b-arts"))); got != 12 {
			t.Errorf("listing has %d articles, want 12", got)
		}
		if !strings.Contains(res.body, `action="/landing?page-arts=2"`) {
			t.Error("form action drops the listing state")
		}
	})

	t.Run("redirects to a confirmation after a valid submission without JavaScript", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing?page-arts=2", validContact(), nil)
		if res.status != http.StatusSeeOther || res.header.Get("Location") != "/landing?page-arts=2&sent=contact" {
			t.Fatalf("status = %d, location = %q", res.status, res.header.Get("Location"))
		}
		if len(inbox.submissions) != 1 || inbox.submissions[0].Fields[3].Value != "Hello\nthere" {
			t.Errorf("submissions = %+v", inbox.submissions)
		}
		confirmation := request(t, http.MethodGet, ts.URL+"/landing?sent=contact", nil, nil)
		if !strings.Contains(confirmation.body, "Message sent") || strings.Contains(confirmation.body, `id="b-contact-form" method="post"`) {
			t.Error("confirmation page does not replace the form with the success message")
		}
	})

	t.Run("returns the form with errors as a fragment to htmx", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing", invalid, enhanced)
		if res.status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d", res.status)
		}
		if !strings.HasPrefix(strings.TrimSpace(res.body), `<form class="base-form" id="b-contact-form"`) || !strings.Contains(res.body, "There is a problem") {
			t.Errorf("body is not the form fragment:\n%s", res.body)
		}
	})

	t.Run("returns the success message as a fragment to htmx", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing", validContact(), enhanced)
		if res.status != http.StatusOK || !strings.Contains(res.body, `id="b-contact-form" tabindex="-1" autofocus`) || strings.Contains(res.body, "<html") {
			t.Errorf("status = %d, body:\n%s", res.status, res.body)
		}
	})

	t.Run("rejects forms of unknown sections", func(t *testing.T) {
		form := validContact()
		form.Set(components.TargetParam, "arts")
		if res := request(t, http.MethodPost, ts.URL+"/landing", form, nil); res.status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", res.status)
		}
	})

	t.Run("rejects cross-origin submissions", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing", validContact(), map[string]string{"Sec-Fetch-Site": "cross-site"})
		if res.status != http.StatusForbidden {
			t.Errorf("status = %d, want 403", res.status)
		}
	})
}

func TestMessageLengthCountsLineBreaksOnce(t *testing.T) {
	ts := newServer(t)
	form := validContact()
	form.Set("message", strings.Repeat("a\r\n", 1000))
	if res := request(t, http.MethodPost, ts.URL+"/landing", form, enhanced); res.status != http.StatusOK {
		t.Errorf("status = %d, want 200 for 2000 characters", res.status)
	}
	form.Set("message", strings.Repeat("a\r\n", 1000)+"ab")
	if res := request(t, http.MethodPost, ts.URL+"/landing", form, enhanced); !strings.Contains(res.body, "Message must be 2,000 characters or fewer") {
		t.Error("message over the limit was accepted")
	}
}

func TestComponentAssets(t *testing.T) {
	ts := newServer(t)
	body := request(t, http.MethodGet, ts.URL+"/landing", nil, nil).body
	head, page, _ := strings.Cut(body, "</head>")

	t.Run("inlines the styles of used components once in the head", func(t *testing.T) {
		// Two listings render six cards each.
		if got := strings.Count(head, ".base-card {"); got != 1 {
			t.Errorf("card styles appear %d times in the head", got)
		}
		if strings.Contains(head, ".block-section-hero {") {
			t.Error("head contains styles of an unused component")
		}
		if strings.Contains(page, "<style>") {
			t.Error("body contains styles")
		}
	})

	t.Run("links one cacheable script for all components", func(t *testing.T) {
		_, src, ok := strings.Cut(head, `<script src="/assets/app.js?v=`)
		if !ok {
			t.Fatal("no script bundle")
		}
		version, _, _ := strings.Cut(src, `"`)
		js := request(t, http.MethodGet, ts.URL+"/assets/app.js?v="+version, nil, nil)
		if !strings.Contains(js.body, "data-character-limit") || !strings.Contains(js.header.Get("Cache-Control"), "immutable") {
			t.Errorf("script bundle: %d %v", js.status, js.header)
		}
	})

	t.Run("leaves styles out of fragments", func(t *testing.T) {
		fragment := request(t, http.MethodGet, ts.URL+"/landing?page-arts=2&_block=arts", nil, enhanced).body
		if strings.Contains(fragment, "<style>") || strings.Contains(fragment, "<script") {
			t.Errorf("fragment repeats assets:\n%s", fragment)
		}
	})
}
