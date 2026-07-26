package nyctereutes

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Omochice/nyctereutes/internal/color"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
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
	code := markerColor(line)
	if code == "" {
		return line
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(code)).Render(line)
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
		return color.Green
	case '-':
		return color.Red
	case '~':
		return color.Yellow
	default:
		return ""
	}
}
