package components

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"

	"storyblok-go-website/internal/storyblok"
)

const (
	// SentParam holds the short ID of the form that was sent successfully, so the
	// page shown after the redirect can confirm it.
	SentParam           = "sent"
	contactNameLimit    = 200
	contactMessageLimit = 2000
)

var (
	contactTopics = []FormOption{
		{Value: "", Label: "Choose a topic"},
		{Value: "sales", Label: "Sales"},
		{Value: "support", Label: "Support"},
		{Value: "partnerships", Label: "Partnerships"},
		{Value: "press", Label: "Press"},
	}
	// Deliberately permissive; the only reliable check is sending a mail.
	emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// BlockSectionContact is a contact form. The CMS provides its copy; the
// fields and their validation are defined here.
type BlockSectionContact struct {
	storyblok.Blok
	SectionStyle
	Heading        string `json:"heading"`
	Text           string `json:"text"`
	SuccessMessage string `json:"success_message"`

	State ContactState `json:"-"`
}

type ContactState struct {
	Path      string
	Values    ContactValues
	Errors    map[string]string
	Sent      bool
	Guard     FormGuardFields
	FormError string
}

type ContactValues struct {
	Name, Email, Topic, Message string
	Consent                     bool
}

func (b *BlockSectionContact) Load(_ context.Context, _ Content, req Request) error {
	// Submitting keeps the state of the page's other blocks.
	b.State.Path = req.URLWithState()
	b.State.Sent = ShortID(b) != "" && req.Query.Get(SentParam) == ShortID(b)
	b.State.Guard = req.FormGuard()
	return nil
}

func (b *BlockSectionContact) Submit(ctx context.Context, inbox Inbox, form url.Values) (bool, error) {
	v := contactValues(form)
	for i, field := range b.State.Guard.Attribution {
		b.State.Guard.Attribution[i].Value = truncate(form.Get(field.Name), maxAttributionChars)
	}
	b.State.Values = v
	b.State.Errors = validateContact(v)
	if len(b.State.Errors) > 0 {
		return false, nil
	}
	err := inbox.Deliver(ctx, Submission{Form: b.Component, Fields: []SubmissionField{
		{Name: "name", Value: v.Name},
		{Name: "email", Value: v.Email},
		{Name: "topic", Value: v.Topic},
		{Name: "message", Value: v.Message},
	}, Attribution: AttributionFrom(form)})
	if err != nil {
		return false, err
	}
	b.State.Sent = true
	return true, nil
}

func contactValues(form url.Values) ContactValues {
	return ContactValues{
		Name:  strings.TrimSpace(form.Get("name")),
		Email: strings.TrimSpace(form.Get("email")),
		Topic: form.Get("topic"),
		// Browsers submit line breaks as CRLF but count them as one character.
		Message: strings.TrimSpace(strings.ReplaceAll(form.Get("message"), "\r\n", "\n")),
		Consent: form.Get("consent") != "",
	}
}

func (b *BlockSectionContact) Confirm() { b.State.Sent = true }

func (b *BlockSectionContact) Reject(form url.Values, message string) {
	b.State.Values = contactValues(form)
	b.State.FormError = message
}

func validateContact(v ContactValues) map[string]string {
	errs := map[string]string{}
	switch {
	case v.Name == "":
		errs["name"] = "Enter your name"
	case characters(v.Name) > contactNameLimit:
		errs["name"] = fmt.Sprintf("Name must be %d characters or fewer", contactNameLimit)
	}
	switch {
	case v.Email == "":
		errs["email"] = "Enter your email address"
	case !emailPattern.MatchString(v.Email):
		errs["email"] = "Enter an email address in the correct format, like name@example.com"
	}
	isTopic := func(o FormOption) bool { return o.Value != "" && o.Value == v.Topic }
	if !slices.ContainsFunc(contactTopics, isTopic) {
		errs["topic"] = "Choose a topic"
	}
	switch {
	case v.Message == "":
		errs["message"] = "Enter your message"
	case characters(v.Message) > contactMessageLimit:
		errs["message"] = fmt.Sprintf("Message must be %s characters or fewer", formatCount(contactMessageLimit))
	}
	if !v.Consent {
		errs["consent"] = "Agree to the privacy policy to send your message"
	}
	return errs
}

// characters counts like JavaScript's String length, which the character
// count shows while typing.
func characters(s string) int { return len(utf16.Encode([]rune(s))) }

func formatCount(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func (b *BlockSectionContact) Success() string {
	return cmp.Or(b.SuccessMessage, "Thanks for your message. We will get back to you within two working days.")
}

// BaseForm returns the form with the submitted values and errors.
func (b *BlockSectionContact) BaseForm() BaseForm {
	id := ElementID(b)
	v, errs := b.State.Values, b.State.Errors
	consent := ""
	if v.Consent {
		consent = "yes"
	}
	return BaseForm{
		ID:     id + "-form",
		Action: b.State.Path,
		Hidden: []FormValue{{Name: TargetParam, Value: ShortID(b)}},
		Guard:  b.State.Guard,
		Error:  b.State.FormError,
		Submit: "Send message",
		Fields: []BaseFormField{
			{ID: id + "-name", Name: "name", Label: "Name", Value: v.Name, Error: errs["name"],
				Control: ControlInput, Autocomplete: "name", Required: true},
			{ID: id + "-email", Name: "email", Label: "Email address", Value: v.Email, Error: errs["email"],
				Hint:    "We only use it to answer your message.",
				Control: ControlInput, Type: "email", Autocomplete: "email", Required: true},
			{ID: id + "-topic", Name: "topic", Label: "Topic", Value: v.Topic, Error: errs["topic"],
				Control: ControlSelect, Options: contactTopics, Required: true},
			{ID: id + "-message", Name: "message", Label: "Message", Value: v.Message, Error: errs["message"],
				Hint:    fmt.Sprintf("You can enter up to %s characters.", formatCount(contactMessageLimit)),
				Control: ControlTextarea, Required: true, CharacterLimit: contactMessageLimit},
			{ID: id + "-consent", Name: "consent", Label: "I agree that my details are stored to answer this message.",
				Value: consent, Error: errs["consent"], Control: ControlCheckbox, Required: true},
		},
	}
}
