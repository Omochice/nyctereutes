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

// ANSI 256-color codes, matching the palette the dep TUI uses so both surfaces
// read alike.
const (
	colorGreen  = "42"
	colorYellow = "226"
	colorRed    = "196"
)

// printChanges writes one project's header followed by its change lines, each
// indented and colored by its diff marker when colorize is set.
func printChanges(out io.Writer, name string, changes []repository.Change, colorize bool) {
	_, _ = fmt.Fprintf(out, "%s\n", name)
	for _, change := range changes {
		for line := range strings.SplitSeq(change.String(), "\n") {
			_, _ = fmt.Fprintf(out, "  %s\n", styleLine(line, colorize))
		}
	}
}

// styleLine colors line by the diff marker it leads with: "+" green, "-" red,
// "~" yellow, so an addition, a removal and an update header read differently
// on one scale. Any other line, and every line when colorize is unset, is
// returned verbatim.
func styleLine(line string, colorize bool) string {
	if !colorize {
		return line
	}
	color := markerColor(line)
	if color == "" {
		return line
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(line)
}

// markerColor returns the color for line's leading diff marker, ignoring the
// indentation a block line carries, or "" when the line has no marker.
func markerColor(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" {
		return ""
	}
	switch trimmed[0] {
	case '+':
		return colorGreen
	case '-':
		return colorRed
	case '~':
		return colorYellow
	default:
		return ""
	}
}

// wantsColor reports whether ANSI color should be written to w: only when w is a
// real terminal and NO_COLOR is absent, so piped or captured output stays plain.
func wantsColor(w io.Writer) bool {
	if noColor() {
		return false
	}
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}

// noColor reports whether the NO_COLOR convention forbids color: the variable is
// present, regardless of its value, even an empty one.
func noColor() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}
