// Package static holds files served unchanged under /assets/.
package static

import "embed"

// FS contains the live preview script and the brand fonts, which are fetched
// by scripts/fetch-fonts.sh because they are licensed and not committed.
//
//go:embed preview.js all:fonts
var FS embed.FS
