// The "doc" subcommand tree, which serves the documentation embedded in the
// binary at build time.
package doc

import "github.com/Omochice/nyctereutes/cli"

// The command tree go-flags parses "doc" and its subcommands into. It has no
// Execute of its own: "doc" alone is a usage error, so the streams are held by
// the subcommands rather than here.
type Command struct {
	List *listCommand `command:"list" description:"list the embedded documents"`
}

// Builds the tree with every subcommand wired to the given streams. It takes no
// glab runner: the documents are compiled in, so nothing here reaches GitLab.
func New(inout *cli.ProcInout) *Command {
	return &Command{
		List: &listCommand{inout: inout},
	}
}
