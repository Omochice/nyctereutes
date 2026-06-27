// Package nyctereutes wires the command-line subcommands onto the cli plumbing.
package nyctereutes

import (
	"errors"
	"fmt"

	flags "github.com/jessevdk/go-flags"

	"github.com/Omochice/nyctereutes/cli"
)

// errNotImplemented marks a subcommand whose real behavior is still a stub.
var errNotImplemented = errors.New("not implemented")

// stubCommand is a subcommand whose real behavior is not implemented yet. Every
// skeleton subcommand shares this single concept until it gains its own logic.
type stubCommand struct {
	inout *cli.ProcInout
}

func (c *stubCommand) Execute(args []string) error {
	fmt.Fprintln(c.inout.Stderr, "not implemented")
	return errNotImplemented
}

type options struct {
	Dep   *stubCommand `command:"dep" description:"manage dependencies"`
	Infra *stubCommand `command:"infra" description:"manage infrastructure"`
	Help  *stubCommand `command:"help" description:"show help"`
}

// MainCommand parses args, dispatches to the requested subcommand, and reports
// the process exit status.
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
			fmt.Fprintln(inout.Stdout, flagsErr.Message)
			return 0
		}
		fmt.Fprintln(inout.Stderr, err)
		parser.WriteHelp(inout.Stderr)
		return 1
	}
	return 0
}
