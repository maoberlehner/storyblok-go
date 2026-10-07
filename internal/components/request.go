package components

import (
	"context"
	"net/url"
	"strings"

	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

// sectioned is a page whose content consists of consecutive sections.
type sectioned interface {
	SectionBlocks() Blocks
}

// Request is what blocks see of the HTTP request they are rendered for.
type Request struct {
	// Path is the page URL path that forms submit to.
	Path    string
	Query   url.Values
	Version storyblok.Version
	// Enhanced is set for htmx requests, which only render a fragment of the
	// targeted block.
	Enhanced bool
	// FormToken is the guard token forms rendered for this request carry.
	FormToken string
	Locale    locale.Locale
}

// TargetParam names the request parameter that holds the short ID of the
// block an enhanced request or a form submission is addressed to.
const TargetParam = "_block"

// shortIDLength keeps URLs readable. Block UIDs are random UUIDs, so their
// first 8 hex digits collide on one page with negligible probability.
const shortIDLength = 8

// ShortID identifies a block within its page in URLs and element IDs.
func ShortID(block Block) string {
	id, _, _ := strings.Cut(block.Meta().UID, "-")
	return id[:min(len(id), shortIDLength)]
}

// Targets reports whether the request is addressed to block.
func (r Request) Targets(block Block) bool {
	id := ShortID(block)
	return id != "" && r.Query.Get(TargetParam) == id
}

// StateQuery returns the query parameters that describe the page's state,
// such as the pages each listing shows. Forms and redirects carry them over,
// so one block's request does not reset the others.
func (r Request) StateQuery() url.Values {
	state := url.Values{}
	for key, values := range r.Query {
		if key != TargetParam && key != SentParam {
			state[key] = values
		}
	}
	return state
}

// URLWithState returns the page path with the state query, minus the given
// parameters.
func (r Request) URLWithState(without ...string) string {
	state := r.StateQuery()
	for _, key := range without {
		state.Del(key)
	}
	if len(state) == 0 {
		return r.Path
	}
	return r.Path + "?" + state.Encode()
}

// stateful is a block that keeps its state in URL query parameters.
type stateful interface {
	StateParams() []string
}

// StateParams lists the query parameters block keeps its state in.
func StateParams(block Block) []string {
	if s, ok := block.(stateful); ok {
		return s.StateParams()
	}
	return nil
}

// Content is the content source blocks load additional data from.
type Content interface {
	Stories(ctx context.Context, opts storyblok.StoriesOptions) (storyblok.StoryList, error)
}

// loader is a block that needs data beyond its own content to render.
type loader interface {
	Load(ctx context.Context, content Content, req Request) error
}

// Load lets block fetch the data it needs beyond its content, if any.
func Load(ctx context.Context, content Content, block Block, req Request) error {
	if l, ok := block.(loader); ok {
		return l.Load(ctx, content, req)
	}
	return nil
}

// LoadSections loads the data of every section of page.
func LoadSections(ctx context.Context, content Content, page Block, req Request) error {
	p, ok := page.(sectioned)
	if !ok {
		return nil
	}
	for _, section := range p.SectionBlocks() {
		if err := Load(ctx, content, section, req); err != nil {
			return err
		}
	}
	return nil
}

// FindSection returns the section of page with the given short ID.
func FindSection(page Block, id string) (Block, bool) {
	p, ok := page.(sectioned)
	if !ok || id == "" {
		return nil, false
	}
	for _, section := range p.SectionBlocks() {
		if ShortID(section) == id {
			return section, true
		}
	}
	return nil, false
}

// Submission is a valid form submission.
type Submission struct {
	Form        string
	Fields      []SubmissionField
	Attribution []SubmissionField
}

type SubmissionField struct {
	Name, Value string
}

// Inbox receives valid form submissions.
type Inbox interface {
	Deliver(ctx context.Context, s Submission) error
}

// FormHandler is a block that accepts form submissions.
type FormHandler interface {
	Block
	// Submit validates the form values and delivers them if they are valid.
	// It keeps the values and errors for rendering the response.
	Submit(ctx context.Context, inbox Inbox, values url.Values) (valid bool, err error)
	// Confirm shows the form as sent without delivering anything; bots get
	// the same response as people.
	Confirm()
	// Reject shows the form again with the submitted values and a
	// form-level error, without delivering anything.
	Reject(values url.Values, message string)
}

// FormGuard returns the hidden fields for forms on this page. UTM parameters
// of the current URL are prefilled, so attribution works without JavaScript
// for submissions from the landing page itself.
func (r Request) FormGuard() FormGuardFields {
	fields := FormGuardFields{Token: r.FormToken}
	for _, name := range AttributionFields {
		value := ""
		if strings.HasPrefix(name, "utm_") {
			value = truncate(r.Query.Get(name), maxAttributionChars)
		}
		fields.Attribution = append(fields.Attribution, FormValue{Name: name, Value: value})
	}
	return fields
}
