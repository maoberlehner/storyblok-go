package components

import (
	"encoding/json/v2"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

// richtext renders a richtext document. Embedded bloks render as components.
func (r *Renderer) richtext(rt storyblok.Richtext) (template.HTML, error) {
	w := richtextWriter{renderer: r}
	if err := w.node(rt.RichtextNode); err != nil {
		return "", err
	}
	return template.HTML(w.String()), nil
}

// richtextInline renders a richtext document without paragraph wrappers, for
// fields that are rendered inside an element like a heading.
func (r *Renderer) richtextInline(rt storyblok.Richtext) (template.HTML, error) {
	w := richtextWriter{renderer: r}
	first := true
	for _, block := range rt.Content {
		if !first {
			w.WriteString("<br>")
		}
		first = false
		if block.Type == "paragraph" || block.Type == "heading" {
			if err := w.children(block); err != nil {
				return "", err
			}
			continue
		}
		if err := w.node(block); err != nil {
			return "", err
		}
	}
	return template.HTML(w.String()), nil
}

type richtextWriter struct {
	strings.Builder
	renderer *Renderer
}

var simpleTags = map[string]string{
	"paragraph":    "p",
	"bullet_list":  "ul",
	"ordered_list": "ol",
	"list_item":    "li",
	"blockquote":   "blockquote",
}

func (w *richtextWriter) node(n storyblok.RichtextNode) error {
	switch n.Type {
	case "doc":
		return w.children(n)
	case "text":
		w.text(n)
		return nil
	case "heading":
		level := min(max(intAttr(n.Attrs, "level"), 1), 6)
		fmt.Fprintf(w, "<h%d>", level)
		if err := w.children(n); err != nil {
			return err
		}
		fmt.Fprintf(w, "</h%d>", level)
		return nil
	case "code_block":
		w.WriteString("<pre><code>")
		err := w.children(n)
		w.WriteString("</code></pre>")
		return err
	case "hard_break":
		w.WriteString("<br>")
		return nil
	case "horizontal_rule":
		w.WriteString("<hr>")
		return nil
	case "image":
		fmt.Fprintf(w, `<img src="%s" alt="%s" loading="lazy">`,
			template.HTMLEscapeString(stringAttr(n.Attrs, "src")),
			template.HTMLEscapeString(stringAttr(n.Attrs, "alt")))
		return nil
	case "emoji":
		w.WriteString(template.HTMLEscapeString(stringAttr(n.Attrs, "emoji")))
		return nil
	case "blok":
		return w.bloks(n.Attrs["body"])
	}
	if tag, ok := simpleTags[n.Type]; ok {
		w.WriteString("<" + tag + ">")
		if err := w.children(n); err != nil {
			return err
		}
		w.WriteString("</" + tag + ">")
		return nil
	}
	return w.children(n)
}

func (w *richtextWriter) children(n storyblok.RichtextNode) error {
	for _, child := range n.Content {
		if err := w.node(child); err != nil {
			return err
		}
	}
	return nil
}

func (w *richtextWriter) bloks(body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var blocks Blocks
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return err
	}
	html, err := w.renderer.renderBlocks(blocks)
	w.WriteString(string(html))
	return err
}

func (w *richtextWriter) text(n storyblok.RichtextNode) {
	var closing []string
	for _, mark := range n.Marks {
		open, close := markTags(mark)
		if open == "" {
			continue
		}
		w.WriteString(open)
		closing = append(closing, close)
	}
	w.WriteString(template.HTMLEscapeString(n.Text))
	for i := len(closing) - 1; i >= 0; i-- {
		w.WriteString(closing[i])
	}
}

// safeCSSValue keeps editor-provided colors and class names from breaking out
// of the attribute they are written to.
var safeCSSValue = regexp.MustCompile(`^[#a-zA-Z0-9(),.% -]+$`)

func markTags(mark storyblok.RichtextNode) (open, close string) {
	switch mark.Type {
	case "bold":
		return "<strong>", "</strong>"
	case "italic":
		return "<em>", "</em>"
	case "underline":
		return "<u>", "</u>"
	case "strike":
		return "<s>", "</s>"
	case "code":
		return "<code>", "</code>"
	case "superscript":
		return "<sup>", "</sup>"
	case "subscript":
		return "<sub>", "</sub>"
	case "link":
		return linkTag(mark.Attrs), "</a>"
	case "anchor":
		return `<span id="` + template.HTMLEscapeString(stringAttr(mark.Attrs, "id")) + `">`, "</span>"
	case "styled":
		if class := stringAttr(mark.Attrs, "class"); safeCSSValue.MatchString(class) {
			return `<span class="` + class + `">`, "</span>"
		}
	case "textStyle":
		if color := stringAttr(mark.Attrs, "color"); safeCSSValue.MatchString(color) {
			return `<span style="color: ` + color + `">`, "</span>"
		}
	case "highlight":
		if color := stringAttr(mark.Attrs, "color"); safeCSSValue.MatchString(color) {
			return `<mark style="background-color: ` + color + `">`, "</mark>"
		}
		return "<mark>", "</mark>"
	}
	return "", ""
}

func linkTag(attrs map[string]any) string {
	link := storyblok.Link{
		LinkType:  stringAttr(attrs, "linktype"),
		URL:       stringAttr(attrs, "href"),
		CachedURL: stringAttr(attrs, "href"),
		Anchor:    stringAttr(attrs, "anchor"),
		Email:     strings.TrimPrefix(stringAttr(attrs, "href"), "mailto:"),
	}
	href := link.Href()
	if !isSafeURL(href) {
		href = "#"
	}
	tag := `<a href="` + template.HTMLEscapeString(href) + `"`
	if target := stringAttr(attrs, "target"); target != "" {
		tag += ` target="` + template.HTMLEscapeString(target) + `"`
		if target == "_blank" {
			tag += ` rel="noopener"`
		}
	}
	return tag + ">"
}

func isSafeURL(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "", "http", "https", "mailto", "tel":
		return true
	}
	return false
}

func stringAttr(attrs map[string]any, key string) string {
	s, _ := attrs[key].(string)
	return s
}

func intAttr(attrs map[string]any, key string) int {
	switch v := attrs[key].(type) {
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}
