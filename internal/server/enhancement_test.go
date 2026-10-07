package server_test

import (
	"io"
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

func TestLoadMore(t *testing.T) {
	ts := newServer(t)

	t.Run("shows the first page with a form for the next one", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing", nil, nil)
		if got := strings.Join(articleTitles(res.body), ","); got != "14,13,12,11,10,9" {
			t.Errorf("articles = %s", got)
		}
		for _, want := range []string{
			"Showing 6 of 14 articles",
			`action="/landing"`,
			`name="_block" value="arts"`,
			`name="page" value="2"`,
		} {
			if !strings.Contains(res.body, want) {
				t.Errorf("body does not contain %q", want)
			}
		}
		if strings.Contains(res.body, "autofocus") {
			t.Error("first page moves focus")
		}
	})

	t.Run("renders all loaded pages without JavaScript and focuses the first new article", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?_block=arts&page=2", nil, nil)
		if got := len(articleTitles(res.body)); got != 12 {
			t.Errorf("got %d articles, want 12", got)
		}
		if !strings.Contains(res.body, `id="b-arts-item-7"`) || !strings.Contains(res.body, `href="/articles/article-8" autofocus>`) {
			t.Error("first new article is not anchored and focused")
		}
		if !strings.Contains(res.body, `name="page" value="3"`) {
			t.Error("missing form for the third page")
		}
	})

	t.Run("returns only the requested page as a fragment to htmx", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?_block=arts&page=2", nil, enhanced)
		if got := strings.Join(articleTitles(res.body), ","); got != "8,7,6,5,4,3" {
			t.Errorf("articles = %s", got)
		}
		if strings.Contains(res.body, "<html") || !strings.Contains(res.body, `<hx-partial hx-target="#b-arts-more" hx-swap="outerHTML">`) {
			t.Errorf("body is not the fragment:\n%s", res.body)
		}
		if got := res.header.Get("HX-Replace-Url"); got != "/landing?_block=arts&page=2" {
			t.Errorf("HX-Replace-Url = %q", got)
		}
		if !strings.Contains(strings.Join(res.header.Values("Vary"), ","), "HX-Request") {
			t.Error("response does not vary by HX-Request")
		}
	})

	t.Run("removes the form after the last page", func(t *testing.T) {
		res := request(t, http.MethodGet, ts.URL+"/landing?_block=arts&page=3", nil, enhanced)
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

	t.Run("redirects to a confirmation after a valid submission without JavaScript", func(t *testing.T) {
		res := request(t, http.MethodPost, ts.URL+"/landing", validContact(), nil)
		if res.status != http.StatusSeeOther || res.header.Get("Location") != "/landing?sent=contact" {
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

func TestAssetDelivery(t *testing.T) {
	cardRule := ".base-card {"
	for _, tt := range []struct {
		mode components.AssetDelivery
		// cardRules counts the base card rule in the page. Six cards render.
		cardRules int
		bundled   bool
	}{
		{components.DeliverBundle, 0, true},
		{components.DeliverHead, 1, false},
		{components.DeliverInline, 6, false},
	} {
		t.Run(string(tt.mode), func(t *testing.T) {
			ts := newServer(t, components.WithAssetDelivery(tt.mode))
			body := request(t, http.MethodGet, ts.URL+"/landing", nil, nil).body
			if got := strings.Count(body, cardRule); got != tt.cardRules {
				t.Errorf("card rule appears %d times, want %d", got, tt.cardRules)
			}
			if got := strings.Contains(body, "/assets/app.css?v="); got != tt.bundled {
				t.Errorf("links bundle = %t, want %t", got, tt.bundled)
			}
			if tt.mode == components.DeliverHead {
				head, _, _ := strings.Cut(body, "</head>")
				if !strings.Contains(head, cardRule) || strings.Contains(head, ".block-section-hero {") {
					t.Error("head does not hold exactly the used component styles")
				}
			}
			fragment := request(t, http.MethodGet, ts.URL+"/landing?_block=arts&page=2", nil, enhanced).body
			if tt.mode != components.DeliverInline && strings.Contains(fragment, "<style>") {
				t.Error("fragment repeats styles the page already has")
			}
		})
	}
}
