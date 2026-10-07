package components

import "storyblok-go-website/internal/storyblok"

// MediaSlotSizes is the rendered width of media in the two-column sections of
// the 72rem page: half of it on wide screens, the full width below 48rem.
const MediaSlotSizes = "(min-width: 72rem) 36rem, (min-width: 48rem) 50vw, 100vw"

type BlockMediaImage struct {
	storyblok.Blok
	Image       storyblok.Asset `json:"image"`
	AspectRatio string          `json:"aspect_ratio"`
	Caption     string          `json:"caption"`
}

func (b *BlockMediaImage) BaseImage(priority bool) BaseImage {
	return BaseImage{Asset: b.Image, Sizes: MediaSlotSizes, Ratio: ParseRatio(b.AspectRatio), Priority: priority}
}
