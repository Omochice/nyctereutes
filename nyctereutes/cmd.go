// Package nyctereutes wires the command-line subcommands onto the cli plumbing.
package nyctereutes

import (
	"errors"
	"fmt"

	flags "github.com/jessevdk/go-flags"

	"github.com/Omochice/nyctereutes/cli"
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
	Dep   *stubCommand `command:"dep" description:"manage dependencies"`
	Infra *stubCommand `command:"infra" description:"manage infrastructure"`
	Help  *stubCommand `command:"help" description:"show help"`
}

// MainCommand parses args, dispatches the matched subcommand, and returns the
// process exit code.
func MainCommand(args []string, inout *cli.ProcInout) int {
	opts := &options{
		Dep:   &stubCommand{inout: inout},
		Infra: &stubCommand{inout: inout},
		Help:  &stubCommand{inout: inout},
	}
	parser := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash)
	parser.Name = "nyctereutes"

	if _, err := parser.ParseArgs(args); err != nil {
		if errors.Is(err, errNotImplemented) {
			return 1
		}
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			_, _ = fmt.Fprintln(inout.Stdout, flagsErr.Message)
			return 0
		}
		_, _ = fmt.Fprintln(inout.Stderr, err)
		parser.WriteHelp(inout.Stderr)
		return 1
	}
	return 0
}
