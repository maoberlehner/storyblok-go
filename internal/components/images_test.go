package components

import (
	"bytes"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"storyblok-go-website/internal/locale"
	"storyblok-go-website/internal/storyblok"
)

const photo = "https://a.storyblok.com/f/1/1000x500/abc/photo.jpg"

func renderBase(t *testing.T, component templ.Component) string {
	t.Helper()
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	_, ctx := r.newRender(locale.Default, nil)
	var out strings.Builder
	if err := component.Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestBaseImageRendersAVIFAndWebPCandidates(t *testing.T) {
	out := renderBase(t, baseImage(BaseImage{
		Asset: storyblok.Asset{Filename: photo, Alt: "A <photo>"}, Sizes: "100vw",
	}))
	for _, want := range []string{
		`<source type="image/avif" srcset="` + photo + `/m/320x0/filters:format(avif) 320w, ` + photo + `/m/480x0/filters:format(avif) 480w, ` + photo + `/m/640x0/filters:format(avif) 640w, ` + photo + `/m/800x0/filters:format(avif) 800w" sizes="100vw">`,
		`src="` + photo + `/m/800x0"`,
		`srcset="` + photo + `/m/320x0 320w, ` + photo + `/m/480x0 480w, ` + photo + `/m/640x0 640w, ` + photo + `/m/800x0 800w"`,
		`alt="A &lt;photo&gt;"`, `width="1000"`, `height="500"`, `loading="lazy"`, `decoding="async"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, "1024w") {
		t.Error("srcset upscales beyond the original width")
	}
}

func TestBaseImageCropsToRatioAndPrioritizes(t *testing.T) {
	out := renderBase(t, baseImage(BaseImage{
		Asset: storyblok.Asset{Filename: photo}, Ratio: ParseRatio("1:1"), Priority: true, Sizes: "50vw",
	}))
	for _, want := range []string{`/m/320x320 320w`, `width="1000"`, `height="1000"`, `loading="eager"`, `fetchpriority="high"`, `alt=""`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
}

func TestBaseImageWithoutKnownSize(t *testing.T) {
	unsized := "https://a.storyblok.com/f/1/abc/photo.jpg"
	out := renderBase(t, baseImage(BaseImage{Asset: storyblok.Asset{Filename: unsized}, Sizes: "100vw"}))
	if strings.Contains(out, "width=") || strings.Contains(out, "height=") {
		t.Errorf("dimensions guessed for an unsized asset:\n%s", out)
	}
	if !strings.Contains(out, unsized+"/m/2560x0 2560w") {
		t.Errorf("srcset skipped widths of an unsized asset:\n%s", out)
	}
	withRatio := renderBase(t, baseImage(BaseImage{Asset: storyblok.Asset{Filename: unsized}, Ratio: ParseRatio("16:9")}))
	if !strings.Contains(withRatio, `width="16" height="9"`) {
		t.Errorf("ratio does not reserve space:\n%s", withRatio)
	}
}

func TestBaseImageKeepsSVGs(t *testing.T) {
	logo := "https://a.storyblok.com/f/1/41x41/abc/logo.svg"
	out := renderBase(t, baseImage(BaseImage{Asset: storyblok.Asset{Filename: logo, Alt: "Logo"}}))
	if strings.Contains(out, "<picture") || !strings.Contains(out, `src="`+logo+`"`) || !strings.Contains(out, `width="41"`) {
		t.Errorf("SVG rendered as raster:\n%s", out)
	}
}

func TestParseRatio(t *testing.T) {
	for in, want := range map[string]Ratio{"16:9": {16, 9}, "4:3": {4, 3}, "original": {}, "": {}, "0:1": {}, "x:y": {}} {
		if got := ParseRatio(in); got != want {
			t.Errorf("ParseRatio(%q) = %v, want %v", in, got, want)
		}
	}
}

func renderStory(t *testing.T, content string) string {
	t.Helper()
	var story storyblok.Story[AnyBlock]
	if err := json.Unmarshal([]byte(`{"name":"P","content":`+content+`}`), &story); err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := r.Page(&out, NewPage(story)); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestOnlyImagesInTheFirstSectionArePrioritized(t *testing.T) {
	image := `{"component":"block-media-image","image":{"filename":"` + photo + `","alt":"Team"},"aspect_ratio":"4:3","caption":"Our team"}`
	out := renderStory(t, `{"component":"page-landing-page","title":"T","description":"D","sections":[
		{"component":"block-section-hero","heading":"First","media":[`+image+`]},
		{"component":"block-section-hero","heading":"Second","media":[`+image+`]}]}`)
	if strings.Count(out, `fetchpriority="high"`) != 1 || strings.Count(out, `loading="lazy"`) != 1 {
		t.Errorf("want exactly one prioritized image:\n%s", out)
	}
	for _, want := range []string{`class="block-media-image"`, `<figcaption class="block-media-image__caption">Our team</figcaption>`, `/m/320x240`, `sizes="` + MediaSlotSizes + `"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestMediaImageWithoutAssetRendersNothing(t *testing.T) {
	out := renderStory(t, `{"component":"page-landing-page","title":"T","description":"D","sections":[
		{"component":"block-section-hero","heading":"H","media":[{"component":"block-media-image"}]}]}`)
	if strings.Contains(out, "<figure") || strings.Contains(out, "<picture") {
		t.Errorf("empty media image rendered:\n%s", out)
	}
}
