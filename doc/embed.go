// The documentation shipped with nyctereutes.
package doc

import "embed"

// The documentation pages, compiled into the binary.
//
//go:embed cmd/*.md
var FS embed.FS
