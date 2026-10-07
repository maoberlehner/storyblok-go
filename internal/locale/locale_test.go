package locale_test

import (
	"testing"

	"storyblok-go-website/internal/locale"
)

func TestFromPath(t *testing.T) {
	for _, tt := range []struct{ path, code, rest string }{
		{"/", "en", ""}, {"/landing/launch", "en", "landing/launch"}, {"/de", "de", ""},
		{"/de/", "de", ""}, {"/de/landing", "de", "landing"}, {"/deutsch", "en", "deutsch"},
	} {
		loc, rest := locale.FromPath(tt.path)
		if loc.Code != tt.code || rest != tt.rest {
			t.Errorf("FromPath(%q) = %s, %q; want %s, %q", tt.path, loc.Code, rest, tt.code, tt.rest)
		}
	}
}

func TestMessages(t *testing.T) {
	if got := locale.German.T("contact.submit"); got != "Nachricht senden" {
		t.Errorf("German submit = %q", got)
	}
	if got := locale.English.T("articles.count", 6, 14); got != "Showing 6 of 14 articles" {
		t.Errorf("English count = %q", got)
	}
	if got := locale.German.T("no.such.key"); got != "no.such.key" {
		t.Errorf("missing key = %q", got)
	}
	if locale.German.FormatNumber(2000) != "2.000" || locale.English.FormatNumber(2000) != "2,000" {
		t.Error("number formatting")
	}
}

// identical are strings that are the same in English and German.
var identical = map[string]bool{
	"contact.name": true, "contact.topic_support": true,
	"form.count_remaining_other": true, "form.count_over_other": true,
}

func TestEveryEnglishMessageIsTranslated(t *testing.T) {
	for _, key := range locale.Keys() {
		if locale.German.T(key) == locale.English.T(key) && !identical[key] {
			t.Errorf("%s is not translated", key)
		}
	}
}
