package nyctereutes

import (
	"bytes"
	"strings"
	"testing"

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
		"+ new repository":     colorGreen,
		"    + c":              colorGreen,
		"    - a":              colorRed,
		"~ description: x → y": colorYellow,
		"  ~ merge_template:":  colorYellow,
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

// A non-file writer such as the buffer used throughout the command tests is
// never a terminal, so color must stay off and captured output plain.
func TestWantsColorFalseForNonTerminal(t *testing.T) {
	if wantsColor(&bytes.Buffer{}) {
		t.Error("wantsColor(*bytes.Buffer) = true, want false for a non-terminal writer")
	}
}

func TestWantsColorFalseWhenNoColorSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if wantsColor(&bytes.Buffer{}) {
		t.Error("wantsColor with NO_COLOR set = true, want false")
	}
}
