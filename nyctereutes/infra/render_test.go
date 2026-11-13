package infra

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/color"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

const ansiEscape = "\x1b["

func TestPrintChangesSingleLinePlain(t *testing.T) {
	var buf bytes.Buffer
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "description", OldValue: "old", NewValue: "new"},
	}
	printChanges(&buf, "group/proj", changes, false)
	want := "group/proj\n  ~ description: old → new\n"
	if buf.String() != want {
		t.Errorf("printChanges single-line = %q, want %q", buf.String(), want)
	}
}

func TestPrintChangesMultilineBlock(t *testing.T) {
	var buf bytes.Buffer
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "merge_commit_template", OldValue: "a\n\nb", NewValue: "c"},
	}
	printChanges(&buf, "group/proj", changes, false)
	want := "group/proj\n" +
		"  ~ merge_commit_template:\n" +
		"      - a\n" +
		"      -\n" +
		"      - b\n" +
		"      + c\n"
	if buf.String() != want {
		t.Errorf("printChanges multiline = %q, want %q", buf.String(), want)
	}
}

// A multi-line block colors removed lines red and added lines green so the two
// sides are told apart rather than washed in one color.
func TestPrintChangesBlockUsesRedAndGreen(t *testing.T) {
	var buf bytes.Buffer
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "merge_commit_template", OldValue: "a", NewValue: "b\nc"},
	}
	printChanges(&buf, "group/proj", changes, true)
	got := buf.String()
	for _, code := range []string{"38;5;196", "38;5;42"} {
		if !strings.Contains(got, code) {
			t.Errorf("colored block = %q, want it to contain SGR %q", got, code)
		}
	}
}

func TestMarkerColor(t *testing.T) {
	cases := map[string]string{
		"+ new repository":     color.Green,
		"    + c":              color.Green,
		"    - a":              color.Red,
		"~ description: x → y": color.Yellow,
		"  ~ merge_template:":  color.Yellow,
		"plain text no marker": "",
	}
	for line, want := range cases {
		if got := markerColor(line); got != want {
			t.Errorf("markerColor(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestStyleLineVerbatimWhenColorizeOff(t *testing.T) {
	if got := styleLine("    - a", false); strings.Contains(got, ansiEscape) {
		t.Errorf("styleLine(colorize=false) leaked an ANSI escape: %q", got)
	}
}

var coloredSpan = regexp.MustCompile(`\x1b\[38;5;(\d+)m([^\x1b]*)\x1b\[0?m`)

type span struct {
	color string
	text  string
}

func coloredSpans(output string) []span {
	matches := coloredSpan.FindAllStringSubmatch(output, -1)
	spans := make([]span, 0, len(matches))
	for _, match := range matches {
		spans = append(spans, span{color: match[1], text: match[2]})
	}
	return spans
}

func renderChanges(changes []repository.Change, colorize bool) string {
	var buf bytes.Buffer
	printChanges(&buf, "group/proj", changes, colorize)
	return buf.String()
}

// A one-line update is not washed in the update color: each side of the arrow
// takes the color a multi-line block gives its removed and added lines.
func TestPrintChangesOneLineUpdateColorsEachSide(t *testing.T) {
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "description", OldValue: "old", NewValue: "new"},
	}
	got := coloredSpans(renderChanges(changes, true))
	want := []span{
		{color: color.Yellow, text: "~ description:"},
		{color: color.Red, text: "old"},
		{color: color.Gray, text: "→"},
		{color: color.Green, text: "new"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("colored spans = %q, want %q", got, want)
	}
}
