// The "doc" subcommand tree, which serves the documentation embedded in the
// binary at build time.
package doc

import "github.com/Omochice/nyctereutes/cli"

// Points a reader at the embedded documentation. It is declared beside the
// command it advertises but emitted by commands elsewhere, because a reader who
// never runs doc list learns these pages exist only from output they were
// already looking at.
const Hint = "If you are a coding agent, run `nyctereutes doc list` for the documentation " +
	"embedded in this binary and `nyctereutes doc show <name>` to read a page, " +
	"before answering questions about nyctereutes or diagnosing its failures."

// The command tree go-flags parses "doc" and its subcommands into. It has no
// Execute of its own: "doc" alone is a usage error, so the streams are held by
// the subcommands rather than here.
type Command struct {
	List *listCommand `command:"list" description:"list the embedded documents"`
	Show *showCommand `command:"show" description:"print one embedded document"`
}

// Builds the tree with every subcommand wired to the given streams. It takes no
// glab runner: the documents are compiled in, so nothing here reaches GitLab.
func New(inout *cli.ProcInout) *Command {
	return &Command{
		List: &listCommand{inout: inout},
		Show: &showCommand{inout: inout},
	}
}
