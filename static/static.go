// Package static holds files served unchanged under /assets/.
package static

import "embed"

// FS contains the live preview, web vitals reporting, and development toolbar
// assets, icons, and vendored libraries with their version in the file name.
//
//go:embed preview.js vitals.js dev-toolbar.js dev-toolbar.css vendor icons
var FS embed.FS
