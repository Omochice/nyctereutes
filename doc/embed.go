// The documentation shipped with nyctereutes. The files live here rather than
// beside the command that serves them because go:embed cannot reach outside
// the directory of the package that declares it, and keeping the markdown at
// the top level is what lets the README link to it.
package doc

import "embed"

// The documentation tree, compiled into the binary so that the pages a build
// serves are the ones written for that build.
//
//go:embed cmd/*.md
var FS embed.FS
