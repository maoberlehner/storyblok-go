package components

import "storyblok-go-website/internal/storyblok"

// BlockSectionContent combines any content blocks with optional media beside
// them.
type BlockSectionContent struct {
	storyblok.Blok
	SectionStyle
	Content       Blocks `json:"content"`
	Media         Blocks `json:"media"`
	MediaPosition string `json:"media_position"`
}

func (b *BlockSectionContent) MediaFirst() bool { return b.MediaPosition == "start" }
