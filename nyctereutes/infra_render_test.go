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

func TestPrintChangesColorsEveryLine(t *testing.T) {
	var buf bytes.Buffer
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "merge_commit_template", OldValue: "a\nb", NewValue: "c"},
	}
	printChanges(&buf, "group/proj", changes, true)
	got := buf.String()
	if !strings.Contains(got, ansiEscape) {
		t.Errorf("printChanges(colorize=true) emitted no ANSI escape: %q", got)
	}
	for _, plain := range []string{"~ merge_commit_template:", "- a", "- b", "+ c"} {
		if !strings.Contains(got, plain) {
			t.Errorf("printChanges(colorize=true) = %q, want it to keep the plain fragment %q", got, plain)
		}
	}
}

func TestStyleLinePlainWhenNoColor(t *testing.T) {
	if got := styleLine("~ description: old → new", ""); strings.Contains(got, ansiEscape) {
		t.Errorf("styleLine with empty color leaked an ANSI escape: %q", got)
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
