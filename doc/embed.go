// The documentation shipped with nyctereutes. The files live here rather than
// beside the command that serves them because go:embed cannot reach outside
// the directory of the package that declares it, and keeping the markdown at
// the top level is what lets the README link to it.
package doc

import "embed"

// The documentation, compiled into the binary so that the pages a build serves
// are the ones written for that build. The pattern matches only this directory,
// which is what keeps a page's name free of any path a reader would have to
// learn before typing it.
//
//go:embed *.md
var FS embed.FS
