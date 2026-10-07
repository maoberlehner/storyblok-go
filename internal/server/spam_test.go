package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"storyblok-go-website/internal/components"
)

func TestFormsCarryGuardAndAttributionFields(t *testing.T) {
	_, body := do(t, http.MethodGet, newServer(t).URL+"/landing?utm_source=news&utm_campaign=fall", "")
	for _, want := range []string{
		`name="hp_leave_empty" tabindex="-1" autocomplete="off"`,
		`name="form_started" value="`,
		`<input type="hidden" name="utm_source" value="news" data-attribution>`,
		`<input type="hidden" name="utm_campaign" value="fall" data-attribution>`,
		`<input type="hidden" name="referrer" value="" data-attribution>`,
		`<script src="/assets/attribution.js" defer></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestHoneypotSubmissionsAreNotDelivered(t *testing.T) {
	inbox := &recordingInbox{}
	ts := newServerWithInbox(t, inbox)
	form := validContact()
	form.Set(components.HoneypotField, "https://spam.example")
	res := request(t, http.MethodPost, ts.URL+"/landing", form, nil)
	if res.status != http.StatusSeeOther || !strings.Contains(res.header.Get("Location"), "sent=contact") {
		t.Errorf("bot was not answered like a person: status %d, Location %q", res.status, res.header.Get("Location"))
	}
	if len(inbox.submissions) != 0 {
		t.Error("spam was delivered")
	}
}

// A secret rotation or an instance without the shared secret must not drop
// real messages silently: people can send again.
func TestSubmissionsWithoutValidTokenCanBeResent(t *testing.T) {
	inbox := &recordingInbox{}
	ts := newServerWithInbox(t, inbox)
	form := validContact()
	form.Set(components.StartedField, "1800000000.forged")
	res := request(t, http.MethodPost, ts.URL+"/landing", form, enhanced)
	if res.status != http.StatusUnprocessableEntity || !strings.Contains(res.body, "Please send it again") || len(inbox.submissions) != 0 {
		t.Errorf("status %d, delivered %d, body:\n%s", res.status, len(inbox.submissions), res.body)
	}
	if !strings.Contains(res.body, `value="`+form.Get("email")+`"`) {
		t.Error("entered values were lost")
	}
}

func TestTooFastSubmissionsCanBeResent(t *testing.T) {
	inbox := &recordingInbox{}
	ts := newServerWithInbox(t, inbox)
	form := validContact()
	form.Set(components.StartedField, components.NewFormGuard([]byte(formSecret), time.Now).Token())
	res := request(t, http.MethodPost, ts.URL+"/landing", form, enhanced)
	if res.status != http.StatusUnprocessableEntity || !strings.Contains(res.body, "Please send it again") || len(inbox.submissions) != 0 {
		t.Errorf("status %d, body:\n%s", res.status, res.body)
	}
	if !strings.Contains(res.body, `value="`+form.Get("email")+`"`) {
		t.Error("entered values were lost")
	}
}

func TestOldFormTokensAreAccepted(t *testing.T) {
	inbox := &recordingInbox{}
	ts := newServerWithInbox(t, inbox)
	form := validContact()
	form.Set(components.StartedField, components.NewFormGuard([]byte(formSecret), func() time.Time { return time.Now().Add(-30 * time.Hour) }).Token())
	if res := request(t, http.MethodPost, ts.URL+"/landing", form, nil); res.status != http.StatusSeeOther || len(inbox.submissions) != 1 {
		t.Errorf("status %d, delivered %d", res.status, len(inbox.submissions))
	}
}

func TestSubmissionsCarryAttribution(t *testing.T) {
	inbox := &recordingInbox{}
	ts := newServerWithInbox(t, inbox)
	form := validContact()
	for key, value := range (url.Values{"utm_source": {"news"}, "referrer": {"https://search.example/"}, "landing_page": {"/landing?utm_source=news"}}) {
		form[key] = value
	}
	request(t, http.MethodPost, ts.URL+"/landing", form, nil)
	if len(inbox.submissions) != 1 {
		t.Fatal("not delivered")
	}
	got := map[string]string{}
	for _, f := range inbox.submissions[0].Attribution {
		got[f.Name] = f.Value
	}
	if got["utm_source"] != "news" || got["referrer"] != "https://search.example/" || got["landing_page"] != "/landing?utm_source=news" {
		t.Errorf("attribution = %v", got)
	}
}
