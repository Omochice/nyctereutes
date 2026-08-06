// The documentation shipped with nyctereutes.
package doc

import (
	"embed"
	"fmt"
	"io/fs"
)

// The directory holding the pages. It is named here and nowhere else, so the
// layout can change without anything outside this package following it.
const dir = "cmd"

//go:embed cmd/*.md
var embedded embed.FS

// The documentation pages, rooted at the directory holding them.
func Pages() fs.FS {
	pages, err := fs.Sub(embedded, dir)
	if err != nil {
		panic(fmt.Sprintf("embedded documentation: %v", err))
	}
	return pages
}
