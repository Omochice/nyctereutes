// Wires the command-line subcommands onto the cli plumbing.
package nyctereutes

import (
	"errors"
	"fmt"

	flags "github.com/jessevdk/go-flags"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
)

var errNotImplemented = errors.New("not implemented")

// Shared stand-in for every subcommand not yet implemented.
type stubCommand struct {
	inout *cli.ProcInout
}

func (c *stubCommand) Execute(_ []string) error {
	_, _ = fmt.Fprintln(c.inout.Stderr, "not implemented")
	return errNotImplemented
}

type options struct {
	Dep   *depCommand  `command:"dep" description:"manage dependencies" subcommands-optional:"true"`
	Infra *stubCommand `command:"infra" description:"manage infrastructure"`
	Help  *stubCommand `command:"help" description:"show help"`
}

// MainCommand is the production entry point; it drives the real glab CLI.
func MainCommand(args []string, inout *cli.ProcInout) int {
	return dispatch(args, inout, glab.ExecRunner{})
}

// dispatch parses args and runs the selected subcommand. The runner is injected
// so tests can drive the commands with a fake glab instead of the real CLI.
func dispatch(args []string, inout *cli.ProcInout, runner glab.Runner) int {
	opts := &options{
		Dep:   newDepCommand(inout, runner),
		Infra: &stubCommand{inout: inout},
		Help:  &stubCommand{inout: inout},
	}
	parser := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash|flags.AllowBoolValues)
	parser.Name = "nyctereutes"

	if _, err := parser.ParseArgs(args); err != nil {
		if errors.Is(err, errNotImplemented) {
			return 1
		}
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) {
			if flagsErr.Type == flags.ErrHelp {
				_, _ = fmt.Fprintln(inout.Stdout, flagsErr.Message)
				return 0
			}
			_, _ = fmt.Fprintln(inout.Stderr, err)
			parser.WriteHelp(inout.Stderr)
			return 1
		}
		// A runtime error returned from a subcommand's Execute; the usage help
		// is unrelated, so only the error itself is reported.
		_, _ = fmt.Fprintln(inout.Stderr, err)
		return 1
	}
	return 0
}
