package components

import (
	"slices"

	"storyblok-go-website/internal/storyblok"
)

func init() {
	register[LogoGrid]("logo_grid")
	register[LogoGridItem]("logo_grid_item")
}

// maxSingleRowLogos is the most logos shown in one row; longer grids wrap
// into rows of multiRowColumns.
const (
	maxSingleRowLogos = 7
	multiRowColumns   = 4
)

type LogoGrid struct {
	storyblok.Blok
	Items Blocks `json:"items"`
}

func (g *LogoGrid) MultiRow() bool { return len(g.Items) > maxSingleRowLogos }

func (g *LogoGrid) Columns() int {
	if g.MultiRow() {
		return multiRowColumns
	}
	return max(len(g.Items), 1)
}

func (g *LogoGrid) NamedLogos() bool {
	return slices.ContainsFunc(blocksOfType[*LogoGridItem](g.Items), func(item *LogoGridItem) bool {
		return item.Name != ""
	})
}

type LogoGridItem struct {
	storyblok.Blok
	Logo storyblok.Asset `json:"logo"`
	Link storyblok.Link  `json:"link"`
	Name string          `json:"name"`
}
