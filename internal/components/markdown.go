package components

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"storyblok-go-website/internal/storyblok"
)

// markdownRenderer omits raw HTML and dangerous link URLs, goldmark's
// defaults, so editors can't inject markup.
var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.Linkify),
	goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(headingShift{}, 100))),
)

// headingShift renders "#" headings as h2: the page title is the only h1.
type headingShift struct{}

func (headingShift) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && h.Level == 1 {
			h.Level = 2
		}
		return ast.WalkContinue, nil
	})
}

func renderMarkdown(source storyblok.Markdown) (template.HTML, error) {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
