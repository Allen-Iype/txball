// Package levels embeds the built-in level files. See package level for
// the format.
package levels

import "embed"

// FS holds the level files, played in name order.
//
//go:embed *.txt
var FS embed.FS
