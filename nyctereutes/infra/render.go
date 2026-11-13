package infra

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Omochice/nyctereutes/internal/color"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

// printChanges writes one project's header followed by its change lines, each
// indented and colored when colorize is set.
func printChanges(out io.Writer, name string, changes []repository.Change, colorize bool) {
	_, _ = fmt.Fprintf(out, "%s\n", name)
	for _, change := range changes {
		for _, line := range change.Lines() {
			_, _ = fmt.Fprintf(out, "  %s\n", styleLine(line, colorize))
		}
	}
}

// styleLine colors line by the diff marker it leads with: "+" green, "-" red,
// "~" yellow, so an addition, a removal and an update header read differently
// on one scale.
// The two values of a one-line update are red and green instead.
func styleLine(line repository.Line, colorize bool) string {
	if !colorize {
		return line.String()
	}
	lead := paint(markerColor(line.Lead), line.Lead)
	if line.Transition == nil {
		return lead
	}
	return strings.Join([]string{
		lead,
		paint(color.Red, line.Transition.Old),
		paint(color.Gray, "→"),
		paint(color.Green, line.Transition.New),
	}, " ")
}

func paint(code, text string) string {
	if code == "" || text == "" {
		return text
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(code)).Render(text)
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
