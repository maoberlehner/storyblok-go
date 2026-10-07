// Package static holds files served unchanged under /assets/.
package static

import "embed"

// FS contains the live preview, web vitals reporting, attribution, and
// development toolbar assets, icons, and vendored libraries with their version in the file name.
//
//go:embed preview.js vitals.js attribution.js dev-toolbar.js dev-toolbar.css vendor icons
var FS embed.FS
