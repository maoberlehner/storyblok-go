package components

import (
	"net/url"
	"testing"
	"time"
)

func TestFormGuard(t *testing.T) {
	rendered := time.Unix(1_800_000_000, 0)
	now := rendered
	guard := NewFormGuard([]byte("secret"), func() time.Time { return now })
	token := guard.Token()

	check := func(values url.Values, after time.Duration) Verdict {
		now = rendered.Add(after)
		return guard.Check(values)
	}
	for _, tt := range []struct {
		name   string
		values url.Values
		after  time.Duration
		want   Verdict
	}{
		{"human", url.Values{StartedField: {token}}, 10 * time.Second, Human},
		{"page cached for a day", url.Values{StartedField: {token}}, 24 * time.Hour, Human},
		{"too fast", url.Values{StartedField: {token}}, time.Second, TooFast},
		{"honeypot", url.Values{StartedField: {token}, HoneypotField: {"https://spam.example"}}, time.Minute, Bot},
		{"no token", url.Values{}, time.Minute, Unverified},
		{"forged time", url.Values{StartedField: {"1799999000" + token[len("1800000000"):]}}, time.Minute, Unverified},
		{"other secret", url.Values{StartedField: {NewFormGuard([]byte("other"), func() time.Time { return rendered }).Token()}}, time.Minute, Unverified},
	} {
		if got := check(tt.values, tt.after); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAttributionFromSubmission(t *testing.T) {
	got := AttributionFrom(url.Values{
		"utm_source": {" newsletter "}, "utm_medium": {""}, "referrer": {"https://news.example/a"}, "name": {"Ada"},
	})
	want := []SubmissionField{{Name: "utm_source", Value: "newsletter"}, {Name: "referrer", Value: "https://news.example/a"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}
