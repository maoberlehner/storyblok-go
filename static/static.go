// Package static holds files served unchanged under /assets/.
package static

import "embed"

// FS contains the live preview and development toolbar assets, and vendored
// libraries with their version in the file name.
//
//go:embed preview.js dev-toolbar.js dev-toolbar.css vendor
var FS embed.FS
