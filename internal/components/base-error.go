package components

import "storyblok-go-website/internal/storyblok"

// ErrorContent is the built-in content of error pages that can't come from
// Storyblok, such as when it is unreachable.
type ErrorContent struct {
	storyblok.Blok
	Heading, Text       string
	HomeHref, HomeLabel string
}

func NewErrorContent(heading, text, homeHref, homeLabel string) *ErrorContent {
	return &ErrorContent{Blok: storyblok.Blok{Component: "base-error"}, Heading: heading, Text: text, HomeHref: homeHref, HomeLabel: homeLabel}
}

func (e *ErrorContent) Metadata() Metadata { return Metadata{Title: e.Heading} }

func (e *ErrorContent) Button() BaseButton { return BaseButton{Href: e.HomeHref, Label: e.HomeLabel} }
