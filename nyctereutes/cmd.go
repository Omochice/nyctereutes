// Wires the command-line subcommands onto the cli plumbing.
package nyctereutes

import (
	"errors"
	"fmt"
	"slices"

	flags "github.com/jessevdk/go-flags"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/nyctereutes/dep"
	"github.com/Omochice/nyctereutes/nyctereutes/doc"
	"github.com/Omochice/nyctereutes/nyctereutes/infra"
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

// Backs the "help" subcommand by re-dispatching with --help, so the usage
// rendering and error handling stay those of the flag instead of a copy.
type helpCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner
}

// Signals that an Execute has already written its own diagnostics to stderr,
// so Dispatch must only translate the failure into a non-zero exit instead of
// reporting it a second time.
var errAlreadyReported = errors.New("failure already reported")

func (c *helpCommand) Execute(args []string) error {
	// Inserted before any "--" terminator, after which PassDoubleDash would
	// demote --help to a positional and really execute the target.
	terminator := slices.Index(args, "--")
	if terminator < 0 {
		terminator = len(args)
	}
	helpArgs := slices.Insert(slices.Clone(args), terminator, "--help")
	if Dispatch(helpArgs, c.inout, c.runner) != 0 {
		return errAlreadyReported
	}
	return nil
}

type options struct {
	Version    bool            `short:"v" long:"version" description:"show version"`
	Dep        *dep.Command    `command:"dep" description:"manage dependencies" subcommands-optional:"true"`
	Infra      *infra.Command  `command:"infra" description:"manage infrastructure"`
	Doc        *doc.Command    `command:"doc" description:"read the embedded documentation"`
	Help       *helpCommand    `command:"help" description:"show help"`
	VersionCmd *versionCommand `command:"version" description:"show version"`
}

// Carries the documentation pointer into every command's usage text. go-flags
// renders the long description of the deepest active command only, so a pointer
// set on the root alone would miss every subcommand's usage error.
func advise(commands []*flags.Command) {
	for _, command := range commands {
		command.LongDescription = doc.Hint
		advise(command.Commands())
	}
}

// The production entry point; it drives the real glab CLI.
func MainCommand(args []string, inout *cli.ProcInout) int {
	return Dispatch(args, inout, glab.ExecRunner{})
}

// Parses args, runs the selected subcommand, and returns the process exit
// code. The glab runner is a parameter because it is the whole command tree's
// only external dependency: production passes the real CLI, and each command
// package's tests pass a fake to drive the tree from the outside.
func Dispatch(args []string, inout *cli.ProcInout, runner glab.Runner) int {
	opts := &options{
		Dep:        dep.New(inout, runner),
		Infra:      infra.New(inout, runner),
		Doc:        doc.New(inout),
		Help:       &helpCommand{inout: inout, runner: runner},
		VersionCmd: &versionCommand{inout: inout},
	}
	parser := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash|flags.AllowBoolValues)
	parser.Name = "nyctereutes"
	parser.LongDescription = doc.Hint
	advise(parser.Commands())
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
		if errors.Is(err, errAlreadyReported) {
			return 1
		}
		if flagsErr, ok := errors.AsType[*flags.Error](err); ok {
			if flagsErr.Type == flags.ErrHelp {
				_, _ = fmt.Fprintln(inout.Stdout, flagsErr.Message)
				return 0
			}
			_, _ = fmt.Fprintln(inout.Stderr, err)
			parser.WriteHelp(inout.Stderr)
			return 1
		}
		// A runtime error returned from a subcommand's Execute; the usage help
		// is unrelated, so it is not rendered.
		_, _ = fmt.Fprintln(inout.Stderr, err)
		return 1
	}
	return 0
}
