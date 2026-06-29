package tui

import (
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// Returned when stdin or stdout is not a terminal, since the interactive view
// would otherwise hang on input or emit control sequences into a pipe.
var errNotInteractive = errors.New("dep requires an interactive terminal; use `dep list` for non-interactive output")

// Starts the interactive program for m, reading keys from in and rendering to
// out. It is the thin I/O boundary the dep command uses in production; the
// model's logic is exercised directly in tests rather than through here. It
// fails fast when in or out is not a terminal.
func Run(m Model, in io.Reader, out io.Writer) error {
	if !isTerminal(in) || !isTerminal(out) {
		return errNotInteractive
	}
	program := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run dep TUI: %w", err)
	}
	return nil
}

// isTerminal reports whether stream is an [*os.File] backed by a terminal.
func isTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}
