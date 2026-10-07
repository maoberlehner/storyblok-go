// Package static holds files served unchanged under /assets/.
package static

import "embed"

// FS contains the live preview and development toolbar assets.
//
//go:embed preview.js dev-toolbar.js dev-toolbar.css
var FS embed.FS
