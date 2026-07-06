// Wires the command-line subcommands onto the cli plumbing.
package nyctereutes

import (
	"errors"
	"fmt"
	"io"

	flags "github.com/jessevdk/go-flags"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
)

var errNotImplemented = errors.New("not implemented")

// The build version, overridden at link time via -ldflags "-X". It falls back
// to a sentinel so binaries built without the flag report an obvious value
// rather than an empty string.
var version = "(devel)"

// Writes the build version as a single line.
func writeVersion(w io.Writer) {
	_, _ = fmt.Fprintln(w, version)
}

// Prints the build version. It backs the "version" subcommand; the same output
// is produced by the top-level --version flag.
type versionCommand struct {
	inout *cli.ProcInout
}

func (c *versionCommand) Execute(_ []string) error {
	writeVersion(c.inout.Stdout)
	return nil
}

// Shared stand-in for every subcommand not yet implemented.
type stubCommand struct {
	inout *cli.ProcInout
}

func (c *stubCommand) Execute(_ []string) error {
	_, _ = fmt.Fprintln(c.inout.Stderr, "not implemented")
	return errNotImplemented
}

type options struct {
	Version bool            `short:"v" long:"version" description:"show version"`
	Dep     *depCommand     `command:"dep" description:"manage dependencies" subcommands-optional:"true"`
	Infra   *infraCommand   `command:"infra" description:"manage infrastructure"`
	Help    *stubCommand    `command:"help" description:"show help"`
	Ver     *versionCommand `command:"version" description:"show version"`
}

// The production entry point; it drives the real glab CLI.
func MainCommand(args []string, inout *cli.ProcInout) int {
	return dispatch(args, inout, glab.ExecRunner{})
}

// Parses args and runs the selected subcommand. The runner is injected
// so tests can drive the commands with a fake glab instead of the real CLI.
func dispatch(args []string, inout *cli.ProcInout, runner glab.Runner) int {
	opts := &options{
		Dep:   newDepCommand(inout, runner),
		Infra: newInfraCommand(inout, runner),
		Help:  &stubCommand{inout: inout},
		Ver:   &versionCommand{inout: inout},
	}
	parser := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash|flags.AllowBoolValues)
	parser.Name = "nyctereutes"

	_, err := parser.ParseArgs(args)
	// --version is a top-level flag, so go-flags still reports the missing
	// subcommand; the flag itself is honored before that error is surfaced.
	if opts.Version {
		writeVersion(inout.Stdout)
		return 0
	}
	if err != nil {
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
