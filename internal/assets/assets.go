// Package assets exposes the embedded slide-viewer template, CSS, and JS.
package assets

import "embed"

//go:embed index.html.tmpl viewer.css viewer.js
var FS embed.FS
