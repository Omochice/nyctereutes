package tui

import (
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
)

// Run starts the interactive program for m, reading keys from in and rendering
// to out. It is the thin I/O boundary the dep command uses in production; the
// model's logic is exercised directly in tests rather than through here.
func Run(m Model, in io.Reader, out io.Writer) error {
	program := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run dep TUI: %w", err)
	}
	return nil
}
