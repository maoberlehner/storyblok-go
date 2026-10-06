package components

import (
	"html"
	"net/url"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[FeatureShowcase]("feature_showcase") }

const defaultVideoAspectRatio = 16.0 / 9.0

type FeatureShowcase struct {
	storyblok.Blok
	Title                storyblok.Richtext `json:"title"`
	Description          storyblok.Richtext `json:"description"`
	CTA                  Blocks             `json:"cta"`
	Image                storyblok.Asset    `json:"image"`
	VideoAsset           storyblok.Asset    `json:"video_asset"`
	ShowCornerDecoration bool               `json:"show_corner_decoration"`
	YouTubeURL           string             `json:"youtube_url"`
	AssetPosition        string             `json:"asset_position"`
}

// AssetFirst reports whether the media is placed before the text.
func (f *FeatureShowcase) AssetFirst() bool { return f.AssetPosition == "right" }

// AspectRatio is the poster image's aspect ratio, used to size video players.
func (f *FeatureShowcase) AspectRatio() float64 {
	w, h := f.Image.Size()
	if w == 0 || h == 0 {
		return defaultVideoAspectRatio
	}
	return float64(w) / float64(h)
}

// YouTubeEmbedURL returns the privacy-enhanced, autoplaying embed URL for
// YouTubeURL, or "" when it is not a recognizable YouTube video link.
func (f *FeatureShowcase) YouTubeEmbedURL() string {
	id := youTubeVideoID(f.YouTubeURL)
	if id == "" {
		return ""
	}
	return "https://www.youtube-nocookie.com/embed/" + url.PathEscape(id) + "?autoplay=1&rel=0"
}

// YouTubePlaceholder is the srcdoc of a lazily loaded YouTube iframe: it shows
// the poster image and a play button, and only navigates to the player once
// clicked, so no third-party scripts load with the page.
func (f *FeatureShowcase) YouTubePlaceholder() string {
	return `<style>` + youTubePlaceholderCSS + `</style>` +
		`<a href="` + html.EscapeString(f.YouTubeEmbedURL()) + `" aria-label="Play video">` +
		`<img src="` + html.EscapeString(f.Image.Resize(1280, 0)) + `" alt="` + html.EscapeString(f.Image.Alt) + `">` +
		`<span>` + playIconSVG + `</span></a>`
}

const (
	youTubePlaceholderCSS = `*{margin:0;padding:0}html,body,a,img{height:100%;width:100%}` +
		`img{object-fit:cover;display:block}a{display:block;position:relative}` +
		`span{position:absolute;inset:0;margin:auto;width:58px;height:58px;border-radius:50%;` +
		`background:#fff;border:2px solid #1f1f1f;display:grid;place-items:center;transition:scale .2s}` +
		`a:hover span{scale:1.1}svg{width:24px;height:24px}`
	playIconSVG = `<svg viewBox="0 0 24 24"><path fill="none" stroke="#1f1f1f" stroke-linecap="round" ` +
		`stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 0 1 3.008-1.728l11.997 6.998a2 2 0 0 1 .003 3.458l-12 7A2 2 0 0 1 5 19z"/></svg>`
)

func youTubeVideoID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	switch host {
	case "youtu.be":
		return strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com", "youtube-nocookie.com":
		if id := u.Query().Get("v"); id != "" {
			return id
		}
		for _, prefix := range []string{"/embed/", "/shorts/", "/live/"} {
			if id, ok := strings.CutPrefix(u.Path, prefix); ok {
				return strings.Trim(id, "/")
			}
		}
	}
	return ""
}
