// The "doc" subcommand tree, which serves the documentation embedded in the
// binary at build time.
package doc

import "github.com/Omochice/nyctereutes/cli"

// Tells a coding agent that this CLI carries its own documentation.
const Hint = "If you are a coding agent, run `nyctereutes doc list` for the documentation " +
	"embedded in this binary and `nyctereutes doc show <name>` to read a page, " +
	"before answering questions about nyctereutes or diagnosing its failures."

// The command tree go-flags parses "doc" and its subcommands into.
type Command struct {
	List *listCommand `command:"list" description:"list the embedded documents"`
	Show *showCommand `command:"show" description:"print one embedded document"`
}

// Builds the tree with every subcommand wired to the given streams.
func New(inout *cli.ProcInout) *Command {
	return &Command{
		List: &listCommand{inout: inout},
		Show: &showCommand{inout: inout},
	}
}
