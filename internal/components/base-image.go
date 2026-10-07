package components

import (
	"fmt"
	"strconv"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

// imageWidths are the srcset candidates, from small phones to wide 2x screens.
var imageWidths = []int{320, 480, 640, 800, 1024, 1280, 1600, 1920, 2560}

// fallbackImageWidth suits browsers that ignore srcset.
const fallbackImageWidth = 1280

// Ratio is an aspect ratio such as 16:9. The zero value keeps the image's own.
type Ratio struct{ W, H int }

// ParseRatio reads "16:9"; anything else, such as "original", is the zero Ratio.
func ParseRatio(s string) Ratio {
	w, h, ok := strings.Cut(s, ":")
	if !ok {
		return Ratio{}
	}
	rw, errW := strconv.Atoi(w)
	rh, errH := strconv.Atoi(h)
	if errW != nil || errH != nil || rw <= 0 || rh <= 0 {
		return Ratio{}
	}
	return Ratio{rw, rh}
}

// BaseImage is a responsive image from the Storyblok image service: AVIF for
// browsers that support it, otherwise WebP or the original format.
type BaseImage struct {
	Asset storyblok.Asset
	// Sizes is the sizes attribute: the image's rendered width per viewport.
	Sizes string
	// Ratio crops the image around its focus point.
	Ratio Ratio
	// Priority loads the image eagerly with high priority, for images likely to
	// be the Largest Contentful Paint.
	Priority bool
}

func (i BaseImage) IsSVG() bool { return i.Asset.IsSVG() }

// widths lists the candidates up to the original width. Unknown originals
// (asset URLs without dimensions) get all candidates.
func (i BaseImage) widths() []int {
	original := i.Asset.Width()
	if original == 0 {
		return imageWidths
	}
	var widths []int
	for _, w := range imageWidths {
		if w <= original {
			widths = append(widths, w)
		}
	}
	if len(widths) == 0 {
		return []int{original}
	}
	return widths
}

func (i BaseImage) url(width int, format string) string {
	height := 0
	if i.Ratio.W > 0 {
		height = width * i.Ratio.H / i.Ratio.W
	}
	return i.Asset.Image(storyblok.ImageOptions{Width: width, Height: height, Format: format})
}

func (i BaseImage) srcSet(format string) string {
	var candidates []string
	for _, w := range i.widths() {
		candidates = append(candidates, fmt.Sprintf("%s %dw", i.url(w, format), w))
	}
	return strings.Join(candidates, ", ")
}

func (i BaseImage) SrcSet() string     { return i.srcSet("") }
func (i BaseImage) AVIFSrcSet() string { return i.srcSet("avif") }

func (i BaseImage) Src() string {
	widths := i.widths()
	src := widths[len(widths)-1]
	for _, w := range widths {
		if w >= fallbackImageWidth {
			src = w
			break
		}
	}
	return i.url(src, "")
}

// dimensions returns the width and height attributes that reserve the image's
// space, or zeros when neither the asset nor the ratio tells.
func (i BaseImage) dimensions() (width, height int) {
	w, h := i.Asset.Size()
	if i.Ratio.W == 0 {
		return w, h
	}
	if w == 0 {
		return i.Ratio.W, i.Ratio.H
	}
	return w, w * i.Ratio.H / i.Ratio.W
}

func (i BaseImage) Width() int  { w, _ := i.dimensions(); return w }
func (i BaseImage) Height() int { _, h := i.dimensions(); return h }
