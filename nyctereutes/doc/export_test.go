package doc

import "io/fs"

// Substitutes the filesystem the subcommands read, so a test can present pages
// the embedded documentation cannot contain. The field stays unexported so
// production callers cannot swap it, and this seam lives in a test file, so it
// exists only in the test binary and never widens the package's API.
func SetPages(c *Command, fsys fs.FS) {
	c.List.pages = fsys
	c.Show.pages = fsys
}
