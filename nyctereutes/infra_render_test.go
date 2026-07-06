package nyctereutes

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

const ansiEscape = "\x1b["

func TestRenderChangePlainWhenColorizeOff(t *testing.T) {
	change := repository.Change{
		Type: repository.ChangeUpdate, Field: "description", OldValue: "old", NewValue: "new",
	}
	got := renderChange(change, false)
	if want := change.String(); got != want {
		t.Errorf("renderChange(colorize=false) = %q, want the verbatim line %q", got, want)
	}
	if strings.Contains(got, ansiEscape) {
		t.Errorf("renderChange(colorize=false) leaked an ANSI escape: %q", got)
	}
}

func TestRenderChangeColorsEachKind(t *testing.T) {
	cases := []struct {
		name   string
		change repository.Change
	}{
		{"create", repository.Change{Type: repository.ChangeCreate}},
		{"update", repository.Change{
			Type: repository.ChangeUpdate, Field: "archived", OldValue: false, NewValue: true,
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := renderChange(testCase.change, true)
			if !strings.Contains(got, ansiEscape) {
				t.Errorf("renderChange(colorize=true) emitted no ANSI escape: %q", got)
			}
			if plain := testCase.change.String(); !strings.Contains(got, plain) {
				t.Errorf("renderChange(colorize=true) = %q, want it to keep the plain line %q", got, plain)
			}
		})
	}
}

func TestPrintChangesLayout(t *testing.T) {
	var buf bytes.Buffer
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "description", OldValue: "old", NewValue: "new"},
	}
	printChanges(&buf, "group/proj", changes, false)
	want := "group/proj\n  ~ description: old → new\n"
	if buf.String() != want {
		t.Errorf("printChanges layout = %q, want %q", buf.String(), want)
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
