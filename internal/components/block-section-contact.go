package components

import (
	"cmp"
	"context"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/a-h/templ"

	"storyblok-go-website/internal/locale"
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
	// Deliberately permissive; the only reliable check is sending a mail.
	emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

var contactTopicValues = []string{"sales", "support", "partnerships", "press"}

func contactTopics(loc locale.Locale) []FormOption {
	options := []FormOption{{Value: "", Label: loc.T("contact.topic_choose")}}
	for _, value := range contactTopicValues {
		options = append(options, FormOption{Value: value, Label: loc.T("contact.topic_" + value)})
	}
	return options
}

// BlockSectionContact is a contact form. The CMS provides its copy; the
// fields and their validation are defined here.
type BlockSectionContact struct {
	storyblok.Blok
	SectionStyle
	Heading        string `json:"heading"`
	Text           string `json:"text"`
	SuccessMessage string `json:"success_message"`
}

// contactView is the form as one request shows it.
type contactView struct {
	*BlockSectionContact
	State ContactState
}

type ContactState struct {
	Path      string
	Values    ContactValues
	Errors    map[string]string
	Sent      bool
	Guard     FormGuardFields
	FormError string
	Locale    locale.Locale
}

type ContactValues struct {
	Name, Email, Topic, Message string
	Consent                     bool
}

func (b *BlockSectionContact) Load(_ context.Context, _ Content, req Request) (Block, error) {
	return &contactView{BlockSectionContact: b, State: ContactState{
		// Submitting keeps the state of the page's other blocks.
		Path:   req.URLWithState(),
		Sent:   ShortID(b) != "" && req.Query.Get(SentParam) == ShortID(b),
		Guard:  req.FormGuard(),
		Locale: req.Locale,
	}}, nil
}

func (b *contactView) Submit(ctx context.Context, inbox Inbox, form url.Values) (bool, error) {
	v := contactValues(form)
	for i, field := range b.State.Guard.Attribution {
		b.State.Guard.Attribution[i].Value = truncate(form.Get(field.Name), maxAttributionChars)
	}
	b.State.Values = v
	b.State.Errors = validateContact(v, b.State.Locale)
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

func (b *contactView) Confirm() { b.State.Sent = true }

func (b *contactView) Reject(form url.Values, message string) {
	b.State.Values = contactValues(form)
	b.State.FormError = message
}

func validateContact(v ContactValues, loc locale.Locale) map[string]string {
	errs := map[string]string{}
	switch {
	case v.Name == "":
		errs["name"] = loc.T("contact.error.name_missing")
	case characters(v.Name) > contactNameLimit:
		errs["name"] = loc.T("contact.error.name_long", contactNameLimit)
	}
	switch {
	case v.Email == "":
		errs["email"] = loc.T("contact.error.email_missing")
	case !emailPattern.MatchString(v.Email):
		errs["email"] = loc.T("contact.error.email_format")
	}
	if !slices.Contains(contactTopicValues, v.Topic) {
		errs["topic"] = loc.T("contact.error.topic")
	}
	switch {
	case v.Message == "":
		errs["message"] = loc.T("contact.error.message_missing")
	case characters(v.Message) > contactMessageLimit:
		errs["message"] = loc.T("contact.error.message_long", loc.FormatNumber(contactMessageLimit))
	}
	if !v.Consent {
		errs["consent"] = loc.T("contact.error.consent")
	}
	return errs
}

// characters counts like JavaScript's String length, which the character
// count shows while typing.
func characters(s string) int { return len(utf16.Encode([]rune(s))) }

func (b *contactView) Success() string {
	return cmp.Or(b.SuccessMessage, b.State.Locale.T("contact.success"))
}

// BaseForm returns the form with the submitted values and errors.
func (b *contactView) BaseForm() BaseForm {
	id := ElementID(b)
	v, errs, loc := b.State.Values, b.State.Errors, b.State.Locale
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
		Submit: loc.T("contact.submit"),
		Fields: []BaseFormField{
			{ID: id + "-name", Name: "name", Label: loc.T("contact.name"), Value: v.Name, Error: errs["name"],
				Control: ControlInput, Autocomplete: "name", Required: true},
			{ID: id + "-email", Name: "email", Label: loc.T("contact.email"), Value: v.Email, Error: errs["email"],
				Hint:    loc.T("contact.email_hint"),
				Control: ControlInput, Type: "email", Autocomplete: "email", Required: true},
			{ID: id + "-topic", Name: "topic", Label: loc.T("contact.topic"), Value: v.Topic, Error: errs["topic"],
				Control: ControlSelect, Options: contactTopics(loc), Required: true},
			{ID: id + "-message", Name: "message", Label: loc.T("contact.message"), Value: v.Message, Error: errs["message"],
				Hint:    loc.T("contact.message_hint", loc.FormatNumber(contactMessageLimit)),
				Control: ControlTextarea, Required: true, CharacterLimit: contactMessageLimit,
				CountMessages: CountMessages{
					RemainingOne: loc.T("form.count_remaining_one"), RemainingOther: loc.T("form.count_remaining_other"),
					OverOne: loc.T("form.count_over_one"), OverOther: loc.T("form.count_over_other"),
				}},
			{ID: id + "-consent", Name: "consent", Label: loc.T("contact.consent"),
				Value: consent, Error: errs["consent"], Control: ControlCheckbox, Required: true},
		},
	}
}

func (b *contactView) Fragment() templ.Component { return blockSectionContactFragment(b) }
