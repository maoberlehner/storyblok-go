package components

import (
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

func init() { register[EnterpriseVideo]("enterprise_video") }

type EnterpriseVideo struct {
	storyblok.Blok
	Headline   string          `json:"headline"`
	Thumbnail  storyblok.Asset `json:"thumbnail"`
	VideoAsset storyblok.Asset `json:"video_asset"`
	YoutubeURL string          `json:"youtube_url"`
	Size       string          `json:"size"`
}

const defaultVideoAspectRatio = 16.0 / 9.0

var youTubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// YouTubeID extracts the video ID from watch, short, embed and youtu.be URLs.
func (v *EnterpriseVideo) YouTubeID() string {
	u, err := url.Parse(strings.TrimSpace(v.YoutubeURL))
	if err != nil {
		return ""
	}
	var id string
	switch host := strings.TrimPrefix(u.Hostname(), "www."); {
	case host == "youtu.be":
		id = strings.Trim(u.Path, "/")
	case strings.HasSuffix(host, "youtube.com") || strings.HasSuffix(host, "youtube-nocookie.com"):
		if id = u.Query().Get("v"); id == "" {
			segments := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(segments) == 2 && (segments[0] == "embed" || segments[0] == "shorts" || segments[0] == "live") {
				id = segments[1]
			}
		}
	}
	if !youTubeID.MatchString(id) {
		return ""
	}
	return id
}

func (v *EnterpriseVideo) MaxWidth() string {
	if v.Size == "large" {
		return "var(--default-container-max-width)"
	}
	return "791px"
}

func (v *EnterpriseVideo) AspectRatio() string {
	ratio := defaultVideoAspectRatio
	if w, h := v.Thumbnail.Size(); w > 0 && h > 0 {
		ratio = float64(w) / float64(h)
	}
	return fmt.Sprintf("%.4f", ratio)
}

func (v *EnterpriseVideo) ThumbnailURL() string {
	if !v.Thumbnail.IsZero() {
		return v.Thumbnail.Resize(1582, 0)
	}
	if id := v.YouTubeID(); id != "" {
		return "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"
	}
	return ""
}

// PlayerDocument is the srcdoc of the YouTube iframe: a thumbnail with a play
// link to the autoplaying embed. YouTube's player only loads once the visitor
// clicks, without any script on the page.
func (v *EnterpriseVideo) PlayerDocument() string {
	id := v.YouTubeID()
	if id == "" {
		return ""
	}
	embed := "https://www.youtube-nocookie.com/embed/" + id + "?autoplay=1&rel=0"
	title := template.HTMLEscapeString(v.Title())
	return `<style>` + videoPlayerStyles + `</style>` +
		`<a href="` + template.HTMLEscapeString(embed) + `" aria-label="Play video: ` + title + `">` +
		`<img src="` + template.HTMLEscapeString(v.ThumbnailURL()) + `" alt="">` +
		`<span>` + playIcon + `</span></a>`
}

func (v *EnterpriseVideo) Title() string {
	if v.Headline != "" {
		return v.Headline
	}
	if v.Thumbnail.Alt != "" {
		return v.Thumbnail.Alt
	}
	return "Video"
}

const playIcon = `<svg viewBox="0 0 24 24" width="24" height="24"><path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 0 1 3.008-1.728l11.997 6.998a2 2 0 0 1 .003 3.458l-12 7A2 2 0 0 1 5 19z"/></svg>`

const videoPlayerStyles = `*{box-sizing:border-box}html,body{height:100%;margin:0;overflow:hidden}` +
	`a{display:block;height:100%;position:relative}` +
	`img{height:100%;object-fit:cover;width:100%}` +
	`span{align-items:center;background:#fff;border:1px solid #ddd;border-radius:50%;color:#1f1f1f;display:flex;height:58px;justify-content:center;left:50%;position:absolute;top:50%;transform:translate(-50%,-50%);transition:transform .2s linear;width:58px}` +
	`a:hover span{border-color:#184db5;color:#184db5;transform:translate(-50%,-50%) scale(1.1)}`
