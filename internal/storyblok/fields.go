package storyblok

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Story[T any] struct {
	ID       int64  `json:"id"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	FullSlug string `json:"full_slug"`
	Content  T      `json:"content"`
}

// Blok holds the fields every nestable or content type component shares.
type Blok struct {
	Component string `json:"component"`
	UID       string `json:"_uid"`
	// Editable is only present in draft content. The Visual Editor uses it to
	// map DOM elements back to bloks.
	Editable string `json:"_editable"`
}

// Meta gives components embedding Blok access to the shared fields.
func (b *Blok) Meta() *Blok { return b }

// EditableOptions decodes the "<!--#storyblok#{...}-->" marker in Editable.
func (b Blok) EditableOptions() (raw string, id string, ok bool) {
	raw, found := strings.CutPrefix(b.Editable, "<!--#storyblok#")
	if !found {
		return "", "", false
	}
	raw = strings.TrimSuffix(raw, "-->")
	var options struct {
		ID  string `json:"id"`
		UID string `json:"uid"`
	}
	if json.Unmarshal([]byte(raw), &options) != nil {
		return "", "", false
	}
	return raw, options.ID + "-" + options.UID, true
}

type Asset struct {
	Filename string `json:"filename"`
	Alt      string `json:"alt"`
	Title    string `json:"title"`
	Focus    string `json:"focus"`
}

func (a *Asset) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Asset
	return unmarshalObjectOrEmpty(dec, (*plain)(a))
}

func (a Asset) IsZero() bool { return a.Filename == "" }

func (a Asset) IsSVG() bool { return strings.HasSuffix(strings.ToLower(a.Filename), ".svg") }

// Size returns the intrinsic dimensions encoded in Storyblok asset URLs
// ("/f/<space>/<width>x<height>/<hash>/<name>").
func (a Asset) Size() (width, height int) {
	for segment := range strings.SplitSeq(a.Filename, "/") {
		w, h, ok := strings.Cut(segment, "x")
		if !ok {
			continue
		}
		width, errW := strconv.Atoi(w)
		height, errH := strconv.Atoi(h)
		if errW == nil && errH == nil {
			return width, height
		}
	}
	return 0, 0
}

func (a Asset) Width() int  { w, _ := a.Size(); return w }
func (a Asset) Height() int { _, h := a.Size(); return h }

// Resize returns an image service URL. A zero width or height keeps the
// aspect ratio. Vector images are returned unchanged.
func (a Asset) Resize(width, height int) string {
	if a.IsZero() || a.IsSVG() {
		return a.Filename
	}
	u := fmt.Sprintf("%s/m/%dx%d", a.Filename, width, height)
	if a.Focus != "" && width > 0 && height > 0 {
		u += "/filters:focal(" + a.Focus + ")"
	}
	return u
}

// SrcSet returns a srcset for the given widths, skipping widths larger than
// the original image.
func (a Asset) SrcSet(widths ...int) string {
	if a.IsZero() || a.IsSVG() {
		return ""
	}
	original := a.Width()
	var candidates []string
	for _, w := range widths {
		if original > 0 && w > original {
			continue
		}
		candidates = append(candidates, fmt.Sprintf("%s %dw", a.Resize(w, 0), w))
	}
	return strings.Join(candidates, ", ")
}

// Link is a multilink field value.
type Link struct {
	LinkType  string `json:"linktype"`
	URL       string `json:"url"`
	CachedURL string `json:"cached_url"`
	Anchor    string `json:"anchor"`
	Target    string `json:"target"`
	Email     string `json:"email"`
}

func (l *Link) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Link
	return unmarshalObjectOrEmpty(dec, (*plain)(l))
}

const homeSlug = "home"

func (l Link) Href() string {
	var href string
	switch l.LinkType {
	case "story":
		slug := strings.Trim(l.CachedURL, "/")
		if slug == homeSlug {
			slug = ""
		}
		href = "/" + slug
	case "email":
		href = "mailto:" + cmp.Or(l.Email, l.URL)
	default:
		href = cmp.Or(l.URL, l.CachedURL)
	}
	if l.Anchor != "" {
		href += "#" + url.PathEscape(l.Anchor)
	}
	return href
}

func (l Link) IsZero() bool { return l.URL == "" && l.CachedURL == "" && l.Email == "" }

func (l Link) IsExternal() bool {
	return strings.HasPrefix(l.Href(), "http://") || strings.HasPrefix(l.Href(), "https://")
}

// Color is the value of a color palette plugin field.
type Color struct {
	Value string `json:"value"`
}

func (c *Color) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Color
	return unmarshalObjectOrEmpty(dec, (*plain)(c))
}

func (c Color) String() string { return c.Value }

type RichtextNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Marks   []RichtextNode `json:"marks"`
	Content []RichtextNode `json:"content"`
}

// Richtext is a richtext field value. Plain strings, which unconverted
// textarea fields still contain, become a single paragraph.
type Richtext struct {
	RichtextNode
}

func (r *Richtext) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case 'n':
		return dec.SkipValue()
	case '"':
		var text string
		if err := json.UnmarshalDecode(dec, &text); err != nil {
			return err
		}
		if text != "" {
			r.RichtextNode = RichtextNode{Type: "doc", Content: []RichtextNode{
				{Type: "paragraph", Content: []RichtextNode{{Type: "text", Text: text}}},
			}}
		}
		return nil
	default:
		return json.UnmarshalDecode(dec, &r.RichtextNode)
	}
}

func (r Richtext) IsEmpty() bool { return !hasText(r.RichtextNode) }

func hasText(n RichtextNode) bool {
	if n.Text != "" || n.Type == "image" || n.Type == "blok" {
		return true
	}
	for _, child := range n.Content {
		if hasText(child) {
			return true
		}
	}
	return false
}

// Relation is a story reference field. It holds the story when the relation
// was resolved and nil when the field only contains the story's UUID.
type Relation[T any] struct {
	Story *Story[T]
}

func (r *Relation[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() != '{' {
		return dec.SkipValue()
	}
	r.Story = new(Story[T])
	return json.UnmarshalDecode(dec, r.Story)
}

func (r Relation[T]) IsZero() bool { return r.Story == nil }

func unmarshalObjectOrEmpty(dec *jsontext.Decoder, out any) error {
	if dec.PeekKind() != '{' {
		return dec.SkipValue()
	}
	return json.UnmarshalDecode(dec, out)
}
