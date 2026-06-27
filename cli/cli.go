// Package cli provides the minimal plumbing shared by every nyctereutes
// command: injectable process streams and the entry-point runner.
package cli

import (
	"io"
	"os"
)

// ProcInout holds the injectable process streams so tests can supply
// in-memory buffers instead of the real standard streams.
type ProcInout struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// NewProcInout returns a ProcInout bound to the real standard streams.
func NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Command runs a subcommand against the given arguments and process streams
// and returns the process exit code.
type Command func(args []string, inout *ProcInout) int

// Run executes c with the real process arguments and streams and exits the
// process with the returned code.
func Run(c Command) {
	os.Exit(c(os.Args[1:], NewProcInout()))
}
