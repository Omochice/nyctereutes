// Package cli provides the minimal plumbing shared by every nyctereutes
// command: injectable process streams and the entry-point runner.
package cli

import (
	"io"
	"os"
)

// ProcInout carries the process streams so commands can be exercised with
// in-memory buffers in tests instead of the real standard streams.
type ProcInout struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// NewProcInout returns a ProcInout bound to the real process streams.
func NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Command is the contract of an entry point: it consumes the arguments and the
// process streams and yields the exit status.
type Command func(args []string, inout *ProcInout) int

// Run dispatches a Command against the real process and exits with its status.
func Run(c Command) {
	os.Exit(c(os.Args[1:], NewProcInout()))
}
