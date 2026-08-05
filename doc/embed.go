// The documentation shipped with nyctereutes. The files live here rather than
// beside the command that serves them because go:embed cannot reach outside
// the directory of the package that declares it, and keeping the markdown at
// the top level is what lets the README link to it.
package doc

import "embed"

// The documentation, compiled into the binary so that the pages a build serves
// are the ones written for that build. The pattern reaches one directory deep
// and no further, which is what lets a page be named after its file alone.
//
//go:embed cmd/*.md
var FS embed.FS
