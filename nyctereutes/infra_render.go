package nyctereutes

import (
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

// ANSI 256-color codes for plan/apply diff lines, matching the palette the dep
// TUI uses so both surfaces read alike.
const (
	colorCreateLine = "42"  // green
	colorUpdateLine = "226" // yellow
)

// printChanges writes one project's header followed by its indented change
// lines, coloring each line by its kind when colorize is set.
func printChanges(w io.Writer, name string, changes []repository.Change, colorize bool) {
	_, _ = fmt.Fprintf(w, "%s\n", name)
	for _, change := range changes {
		_, _ = fmt.Fprintf(w, "  %s\n", renderChange(change, colorize))
	}
}

// renderChange returns change's line, colored by its kind when colorize is set
// and returned verbatim otherwise.
func renderChange(change repository.Change, colorize bool) string {
	line := change.String()
	if !colorize {
		return line
	}
	switch change.Type {
	case repository.ChangeCreate:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorCreateLine)).Render(line)
	case repository.ChangeUpdate:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorUpdateLine)).Render(line)
	default:
		return line
	}
}

// wantsColor reports whether ANSI color should be written to w: only when w is a
// real terminal and NO_COLOR is unset, so piped or captured output stays plain.
func wantsColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}
