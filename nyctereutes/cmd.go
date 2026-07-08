// Wires the command-line subcommands onto the cli plumbing.
package nyctereutes

import (
	"errors"
	"fmt"

	flags "github.com/jessevdk/go-flags"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
)

// Build version, stamped in at link time via -ldflags "-X"; the sentinel marks
// an un-stamped build.
var version = "(devel)"

// Backs the "version" subcommand.
type versionCommand struct {
	inout *cli.ProcInout
}

func (c *versionCommand) Execute(_ []string) error {
	_, _ = fmt.Fprintln(c.inout.Stdout, version)
	return nil
}

// Backs the "help" subcommand by re-dispatching the requested command line
// with --help appended, so the printed usage and the error handling stay
// identical to the --help flag instead of duplicating them here.
type helpCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner
}

func (c *helpCommand) Execute(args []string) error {
	dispatch(append(args, "--help"), c.inout, c.runner)
	return nil
}

type options struct {
	Version    bool            `short:"v" long:"version" description:"show version"`
	Dep        *depCommand     `command:"dep" description:"manage dependencies" subcommands-optional:"true"`
	Infra      *infraCommand   `command:"infra" description:"manage infrastructure"`
	Help       *helpCommand    `command:"help" description:"show help"`
	VersionCmd *versionCommand `command:"version" description:"show version"`
}

// The production entry point; it drives the real glab CLI.
func MainCommand(args []string, inout *cli.ProcInout) int {
	return dispatch(args, inout, glab.ExecRunner{})
}

// Parses args and runs the selected subcommand. The runner is injected
// so tests can drive the commands with a fake glab instead of the real CLI.
func dispatch(args []string, inout *cli.ProcInout, runner glab.Runner) int {
	opts := &options{
		Dep:        newDepCommand(inout, runner),
		Infra:      newInfraCommand(inout, runner),
		Help:       &helpCommand{inout: inout, runner: runner},
		VersionCmd: &versionCommand{inout: inout},
	}
	parser := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash|flags.AllowBoolValues)
	parser.Name = "nyctereutes"
	// With --version set, go-flags would still run the subcommand (and its side
	// effects) during ParseArgs, so skip execution here.
	parser.CommandHandler = func(command flags.Commander, cmdArgs []string) error {
		if opts.Version {
			return nil
		}
		return command.Execute(cmdArgs)
	}

	_, err := parser.ParseArgs(args)
	// Bare --version yields the expected ErrCommandRequired; any other parse
	// error (e.g. an unknown flag) must still surface instead of being masked.
	if opts.Version {
		var flagsErr *flags.Error
		if err == nil || (errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrCommandRequired) {
			_, _ = fmt.Fprintln(inout.Stdout, version)
			return 0
		}
	}
	if err != nil {
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
