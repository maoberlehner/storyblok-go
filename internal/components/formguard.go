package components

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// HoneypotField is a text input hidden from people; bots fill it in. Its
	// name matches no autofill heuristic, so browsers and extensions leave it
	// empty.
	HoneypotField = "hp_leave_empty"
	// StartedField holds the signed time the form was rendered.
	StartedField = "form_started"
	// MinFillTime is faster than people fill in a form, even with autofill.
	MinFillTime = 3 * time.Second
	// There is no maximum age: pages are cached for up to a day.

	maxAttributionChars = 500
)

// AttributionFields describe where a visitor came from: the UTM parameters,
// the external referrer, and the first page of the visit.
var AttributionFields = []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "referrer", "landing_page"}

type Verdict int

const (
	Human Verdict = iota
	Bot
	// TooFast submissions may come from people; they can send again.
	TooFast
	// Unverified submissions lack a valid token, e.g. after a secret change
	// while pages with old tokens are cached. They may come from people too.
	Unverified
)

// FormGuard tells people from bots with a honeypot and a signed render time,
// without CAPTCHAs or third parties.
type FormGuard struct {
	secret []byte
	now    func() time.Time
}

func NewFormGuard(secret []byte, now func() time.Time) FormGuard {
	return FormGuard{secret: secret, now: now}
}

// Token is the value of StartedField for a form rendered now.
func (g FormGuard) Token() string {
	ts := strconv.FormatInt(g.now().Unix(), 10)
	return ts + "." + g.sign(ts)
}

func (g FormGuard) sign(ts string) string {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(ts))
	return hex.EncodeToString(mac.Sum(nil))
}

func (g FormGuard) Check(values url.Values) Verdict {
	if values.Get(HoneypotField) != "" {
		return Bot
	}
	ts, signature, ok := strings.Cut(values.Get(StartedField), ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(g.sign(ts))) {
		return Unverified
	}
	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return Unverified
	}
	if g.now().Sub(time.Unix(seconds, 0)) < MinFillTime {
		return TooFast
	}
	return Human
}

// FormGuardFields are the hidden fields every form renders.
type FormGuardFields struct {
	Token       string
	Attribution []FormValue
}

// AttributionFrom returns the non-empty attribution values of a submission.
func AttributionFrom(values url.Values) []SubmissionField {
	var fields []SubmissionField
	for _, name := range AttributionFields {
		if value := truncate(strings.TrimSpace(values.Get(name)), maxAttributionChars); value != "" {
			fields = append(fields, SubmissionField{Name: name, Value: value})
		}
	}
	return fields
}

func truncate(s string, maxChars int) string {
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	return string([]rune(s)[:maxChars])
}
