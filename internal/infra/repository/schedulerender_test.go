package repository

import (
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// A schedule carries several attributes, so a one-line arrow would be unreadable
// for a create. The name goes on a marked header and the attributes below it.
func TestChangeStringRendersAScheduleCreateAsABlock(t *testing.T) {
	change := Change{
		Type: ChangeCreate, Name: "group/proj", Field: fieldPipelineSchedule,
		NewValue: manifest.PipelineSchedule{
			Description: "weekly report", Ref: "refs/heads/main",
			Cron: "0 9 * * 1", CronTimezone: "Asia/Tokyo", Active: true,
		},
	}

	lines := strings.Split(change.String(), "\n")
	if want := `+ pipeline_schedule "weekly report":`; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	wanted := []string{
		"ref:", "refs/heads/main", "cron:", "0 9 * * 1",
		"cron_timezone:", "Asia/Tokyo", "active:", "true",
	}
	for _, want := range wanted {
		if !strings.Contains(change.String(), want) {
			t.Errorf("create block missing %q\n%s", want, change)
		}
	}
}

// Only the attributes that differ are shown, so a plan does not restate the
// ones already matching.
func TestChangeStringRendersAScheduleUpdateAsChangedAttributesOnly(t *testing.T) {
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: LiveSchedule{
			ID: 1, Description: "nightly", Ref: "refs/heads/main",
			Cron: "0 3 * * *", CronTimezone: "UTC", Active: true,
		},
		NewValue: manifest.PipelineSchedule{
			Description: "nightly", Ref: "refs/heads/main",
			Cron: "0 4 * * *", CronTimezone: "UTC", Active: true,
		},
	}

	rendered := change.String()
	lines := strings.Split(rendered, "\n")
	if want := `~ pipeline_schedule "nightly":`; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	if len(lines) != 2 {
		t.Fatalf("rendered %d lines, want a header and one attribute\n%s", len(lines), rendered)
	}
	for _, want := range []string{"cron:", "0 3 * * *", "0 4 * * *"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("update block missing %q\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "ref:") {
		t.Errorf("update block restates an unchanged attribute\n%s", rendered)
	}
}

// A delete has nothing to show beyond which schedule goes, so it stays on one
// line. Without its own case the switch would have emitted an empty string.
func TestChangeStringRendersAScheduleDeleteOnOneLine(t *testing.T) {
	change := Change{
		Type: ChangeDelete, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: LiveSchedule{ID: 1, Description: "old nightly"},
	}

	if want := `- pipeline_schedule "old nightly"`; change.String() != want {
		t.Errorf("rendered = %q, want %q", change.String(), want)
	}
}

// A project create is still rendered the way it was; only schedule changes take
// the new shape.
func TestChangeStringStillRendersAProjectCreate(t *testing.T) {
	change := Change{Type: ChangeCreate, Name: "group/proj", Field: fieldRepository, NewValue: "group/proj"}
	if want := "+ new repository"; change.String() != want {
		t.Errorf("rendered = %q, want %q", change.String(), want)
	}
}

// The delete-refusal that used to report a schedule's variables is gone, so the
// plan line itself has to name them: their values exist only on GitLab, and
// nothing else in a plan mentions them.
func TestChangeStringNamesTheVariablesADeleteDestroys(t *testing.T) {
	change := Change{
		Type: ChangeDelete, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: LiveSchedule{ID: 1, Description: "nightly", Variables: []ScheduleVariable{
			{Key: "TARGET", Value: "prod"},
			{Key: "DEPLOY_TOKEN", Value: "s3cr3t"},
		}},
	}

	rendered := change.String()
	for _, want := range []string{`- pipeline_schedule "nightly":`, "variables:", "- DEPLOY_TOKEN", "- TARGET"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered = %q, want it to contain %q", rendered, want)
		}
	}
	for _, leaked := range []string{"prod", "s3cr3t"} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("rendered leaks the value %q\n%s", leaked, rendered)
		}
	}
}
