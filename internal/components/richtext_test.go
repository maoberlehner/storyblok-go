package components

import (
	"encoding/json/v2"
	"testing"

	"storyblok-go-website/internal/storyblok"
)

func renderRichtext(t *testing.T, doc string, inline bool) string {
	t.Helper()
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	var rt storyblok.Richtext
	if err := json.Unmarshal([]byte(doc), &rt); err != nil {
		t.Fatal(err)
	}
	render := r.richtext
	if inline {
		render = r.richtextInline
	}
	html, err := render(rt)
	if err != nil {
		t.Fatal(err)
	}
	return string(html)
}

func TestRichtext(t *testing.T) {
	tests := []struct {
		name   string
		doc    string
		inline bool
		want   string
	}{
		{
			name: "marks and escaping",
			doc:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Fast <b>","marks":[{"type":"bold"},{"type":"styled","attrs":{"class":"heading-emphasis-solid-blue"}}]}]}]}`,
			want: `<p><strong><span class="heading-emphasis-solid-blue">Fast &lt;b&gt;</span></strong></p>`,
		},
		{
			name: "story links",
			doc:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Docs","marks":[{"type":"link","attrs":{"href":"docs/guide","linktype":"story","anchor":"start"}}]}]}]}`,
			want: `<p><a href="/docs/guide#start">Docs</a></p>`,
		},
		{
			name: "unsafe links",
			doc:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)","linktype":"url"}}]}]}]}`,
			want: `<p><a href="#">x</a></p>`,
		},
		{
			name:   "inline drops paragraph wrappers",
			doc:    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"One"}]},{"type":"paragraph","content":[{"type":"text","text":"Two"}]}]}`,
			inline: true,
			want:   `One<br>Two`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderRichtext(t, tt.doc, tt.inline); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}
