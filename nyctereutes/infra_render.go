package nyctereutes

import (
	"fmt"
	"io"
	"os"
	"strings"

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

// printChanges writes one project's header followed by its change lines, each
// indented and colored by its kind when colorize is set. A change spanning
// several lines has every line indented and colored alike.
func printChanges(out io.Writer, name string, changes []repository.Change, colorize bool) {
	_, _ = fmt.Fprintf(out, "%s\n", name)
	for _, change := range changes {
		color := ""
		if colorize {
			color = lineColor(change.Type)
		}
		for line := range strings.SplitSeq(change.String(), "\n") {
			_, _ = fmt.Fprintf(out, "  %s\n", styleLine(line, color))
		}
	}
}

// lineColor returns the ANSI color for a change kind, or "" for kinds left
// uncolored.
func lineColor(changeType repository.ChangeType) string {
	switch changeType {
	case repository.ChangeCreate:
		return colorCreateLine
	case repository.ChangeUpdate:
		return colorUpdateLine
	default:
		return ""
	}
}

// styleLine renders line in the given ANSI color, or verbatim when color is "".
func styleLine(line, color string) string {
	if color == "" {
		return line
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(line)
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
