package infra

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/color"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
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
		"~ description:":       color.Yellow,
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
	if got := styleLine(repository.Line{Lead: "    - a"}, false); strings.Contains(got, ansiEscape) {
		t.Errorf("styleLine(colorize=false) leaked an ANSI escape: %q", got)
	}
}

var (
	coloredSpan = regexp.MustCompile(`\x1b\[38;5;(\d+)m([^\x1b]*)\x1b\[0?m`)
	anySGR      = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

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

func TestPrintChangesScheduleAttributeUpdateColorsEachSide(t *testing.T) {
	changes := []repository.Change{{
		Type: repository.ChangeUpdate,
		Schedule: &repository.ScheduleChange{
			Description: "nightly",
			Desired:     manifest.PipelineSchedule{Cron: "0 4 * * *"},
			Live:        manifest.PipelineSchedule{Cron: "0 3 * * *"},
		},
	}}
	got := coloredSpans(renderChanges(changes, true))
	want := []span{
		{color: color.Yellow, text: `~ pipeline_schedule "nightly"`},
		{color: color.Yellow, text: "    ~ cron:"},
		{color: color.Red, text: "0 3 * * *"},
		{color: color.Gray, text: "→"},
		{color: color.Green, text: "0 4 * * *"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("colored spans = %q, want %q", got, want)
	}
}

func TestPrintChangesProtectedBranchAttributeUpdateColorsEachSide(t *testing.T) {
	admin := manifest.BranchUnprotectAccessLevel("admin")
	changes := []repository.Change{{
		Type: repository.ChangeUpdate,
		ProtectedBranch: &repository.ProtectedBranchChange{
			Name:    "main",
			Desired: manifest.ProtectedBranch{UnprotectAccessLevel: &admin},
		},
	}}
	got := coloredSpans(renderChanges(changes, true))
	want := []span{
		{color: color.Yellow, text: `~ protected_branch "main"`},
		{color: color.Yellow, text: "    ~ unprotect_access_level:"},
		{color: color.Red, text: "(none)"},
		{color: color.Gray, text: "→"},
		{color: color.Green, text: "admin"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("colored spans = %q, want %q", got, want)
	}
}

func TestPrintChangesWritesNoColorAroundAnEmptyValue(t *testing.T) {
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "description", OldValue: "", NewValue: "new"},
	}
	got := coloredSpans(renderChanges(changes, true))
	want := []span{
		{color: color.Yellow, text: "~ description:"},
		{color: color.Gray, text: "→"},
		{color: color.Green, text: "new"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("colored spans = %q, want %q", got, want)
	}
}

func TestPrintChangesSplitsAValueHoldingAnArrowAtTheRealBoundary(t *testing.T) {
	changes := []repository.Change{
		{Type: repository.ChangeUpdate, Field: "description", OldValue: "a → b", NewValue: "c"},
	}
	got := coloredSpans(renderChanges(changes, true))
	want := []span{
		{color: color.Yellow, text: "~ description:"},
		{color: color.Red, text: "a → b"},
		{color: color.Gray, text: "→"},
		{color: color.Green, text: "c"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("colored spans = %q, want %q", got, want)
	}
}

func TestPrintChangesColorLeavesTheTextUnchanged(t *testing.T) {
	admin := manifest.BranchUnprotectAccessLevel("admin")
	cases := map[string]repository.Change{
		"field update": {
			Type: repository.ChangeUpdate, Field: "description", OldValue: "old", NewValue: "new",
		},
		"value holding an arrow": {
			Type: repository.ChangeUpdate, Field: "description", OldValue: "a → b", NewValue: "c",
		},
		"update from empty": {
			Type: repository.ChangeUpdate, Field: "description", OldValue: "", NewValue: "new",
		},
		"multi-line block": {
			Type: repository.ChangeUpdate, Field: "merge_commit_template", OldValue: "a\n\nb", NewValue: "c",
		},
		"new repository": {Type: repository.ChangeCreate},
		"schedule update": {
			Type: repository.ChangeUpdate,
			Schedule: &repository.ScheduleChange{
				Description: "nightly",
				Desired:     manifest.PipelineSchedule{Cron: "0 4 * * *"},
				Live:        manifest.PipelineSchedule{Cron: "0 3 * * *"},
			},
		},
		"schedule removal": {
			Type:     repository.ChangeDelete,
			Schedule: &repository.ScheduleChange{Description: "nightly", VariableKeys: []string{"TOKEN"}},
		},
		"protected branch update": {
			Type: repository.ChangeUpdate,
			ProtectedBranch: &repository.ProtectedBranchChange{
				Name:    "main",
				Desired: manifest.ProtectedBranch{UnprotectAccessLevel: &admin},
			},
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			changes := []repository.Change{change}
			colored := renderChanges(changes, true)
			if !strings.Contains(colored, ansiEscape) {
				t.Fatalf("colored output = %q, want it to carry color", colored)
			}
			plain := renderChanges(changes, false)
			if got := anySGR.ReplaceAllString(colored, ""); got != plain {
				t.Errorf("colored output without its escapes = %q, want %q", got, plain)
			}
		})
	}
}
