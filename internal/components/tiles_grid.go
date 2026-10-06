package components

import "storyblok-go-website/internal/storyblok"

func init() { register[TilesGrid]("tiles_grid") }

type TilesGrid struct {
	storyblok.Blok
	Tiles Blocks `json:"tiles"`
}
