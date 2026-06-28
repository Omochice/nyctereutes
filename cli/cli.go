// Minimal plumbing shared by every nyctereutes command: injectable process
// streams and the entry-point runner.
package cli

import (
	"io"
	"os"
)

// Injectable process streams: tests can supply in-memory buffers instead of
// the real standard streams.
type ProcInout struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// NewProcInout returns a ProcInout wired to the real standard streams.
func NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Command is a subcommand entry point: it runs against the given arguments and
// streams and returns the process exit code.
type Command func(args []string, inout *ProcInout) int

// Run executes c against the real process arguments and standard streams, then
// exits with the code it returns.
func Run(c Command) {
	os.Exit(c(os.Args[1:], NewProcInout()))
}
