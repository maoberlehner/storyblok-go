package components

import (
	"context"
	"net/url"

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
}

// TargetParam names the request parameter that holds the UID of the block a
// form or link targets.
const TargetParam = "_block"

// Targets reports whether the request is addressed to block.
func (r Request) Targets(block Block) bool {
	return block.Meta().UID != "" && r.Query.Get(TargetParam) == block.Meta().UID
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

// FindSection returns the section of page with the given UID.
func FindSection(page Block, uid string) (Block, bool) {
	p, ok := page.(sectioned)
	if !ok || uid == "" {
		return nil, false
	}
	for _, section := range p.SectionBlocks() {
		if section.Meta().UID == uid {
			return section, true
		}
	}
	return nil, false
}

// Submission is a valid form submission.
type Submission struct {
	Form   string
	Fields []SubmissionField
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
}
