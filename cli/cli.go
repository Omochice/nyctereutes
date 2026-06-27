// Package cli provides the minimal plumbing shared by every nyctereutes
// command: injectable process streams and the entry-point runner.
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

func NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

type Command func(args []string, inout *ProcInout) int

func Run(c Command) {
	os.Exit(c(os.Args[1:], NewProcInout()))
}
