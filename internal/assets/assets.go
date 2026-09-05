// Package assets embeds the default adhan so a fresh install works offline
// with no extra download step.
package assets

import _ "embed"

// DefaultAdhan is the bundled call to prayer, written to the sounds directory
// on first run. Selecting between reciters is a later feature; today there is
// one sound and this is it.
//
//go:embed adhan.mp3
var DefaultAdhan []byte

// DefaultAdhanName is the filename used inside the sounds directory.
const DefaultAdhanName = "default.mp3"
