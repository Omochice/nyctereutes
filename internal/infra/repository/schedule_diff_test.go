package repository

import (
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// A document declaring the given schedules. A nil slice is one carrying no
// pipeline_schedules key at all.
func declaring(schedules []manifest.PipelineSchedule) *manifest.Repository {
	return &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: ownerGroup, Name: nameProj},
		Spec:     manifest.RepositorySpec{PipelineSchedules: schedules},
	}
}

// Live state holding the given schedules. A nil slice is a project whose
// schedules were not read.
func holding(schedules []LiveSchedule) *CurrentState {
	return &CurrentState{Owner: ownerGroup, Name: nameProj, PipelineSchedules: schedules}
}

// One schedule as a decoded document holds it: the timezone and the active flag
// are filled because the decoder seeds what GitLab defaults on create.
func declaredSchedule(description, cron string) manifest.PipelineSchedule {
	return manifest.PipelineSchedule{
		Description:  description,
		Ref:          "refs/heads/main",
		Cron:         cron,
		CronTimezone: "UTC",
		Active:       true,
	}
}

// The same schedule as GitLab reports it.
func liveSchedule(id int, description, cron string) LiveSchedule {
	return LiveSchedule{
		ID:           id,
		Description:  description,
		Ref:          "refs/heads/main",
		Cron:         cron,
		CronTimezone: "UTC",
		Active:       true,
	}
}

// The single schedule change in changes, failing the test when there is not
// exactly one to read.
func onlyScheduleChange(t *testing.T, changes []Change) Change {
	t.Helper()
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly 1", changes)
	}
	if changes[0].Schedule == nil {
		t.Fatalf("change = %+v, want one carrying a schedule", changes[0])
	}
	return changes[0]
}

// A document with no pipeline_schedules key says nothing about schedules, so
// the live ones are left alone rather than removed as undeclared.
func TestDiffLeavesLiveSchedulesAloneWhenTheDocumentDeclaresNoKey(t *testing.T) {
	changes := Diff(declaring(nil), holding([]LiveSchedule{liveSchedule(1, "nightly", "0 3 * * *")}))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// An empty list is a declaration that the project holds no schedule, which is
// the opposite of omitting the key: every live schedule is planned for removal.
func TestDiffPlansEveryLiveScheduleRemovedByAnEmptyList(t *testing.T) {
	live := []LiveSchedule{liveSchedule(1, "nightly", "0 3 * * *"), liveSchedule(2, "weekly", "0 9 * * 1")}

	changes := Diff(declaring([]manifest.PipelineSchedule{}), holding(live))

	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want 2 deletions", changes)
	}
	for index, change := range changes {
		if change.Type != ChangeDelete || change.Schedule == nil {
			t.Fatalf("changes[%d] = %+v, want a schedule deletion", index, change)
		}
		if change.Schedule.ID != live[index].ID {
			t.Errorf("changes[%d].Schedule.ID = %d, want %d", index, change.Schedule.ID, live[index].ID)
		}
	}
}

// A project whose schedules were not read plans no schedule change, however
// much the document declares: a plan does not speak about what it has not seen.
func TestDiffSaysNothingAboutSchedulesThatWereNotRead(t *testing.T) {
	declared := []manifest.PipelineSchedule{declaredSchedule("nightly", "0 3 * * *")}

	changes := Diff(declaring(declared), holding(nil))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// A declared schedule GitLab does not hold is created, carrying no id because
// GitLab has not assigned one.
func TestDiffPlansADeclaredScheduleWithNoLiveCounterpartAsACreation(t *testing.T) {
	want := declaredSchedule("nightly", "0 3 * * *")

	changes := Diff(declaring([]manifest.PipelineSchedule{want}), holding([]LiveSchedule{}))

	change := onlyScheduleChange(t, changes)
	if change.Type != ChangeCreate || change.Field != fieldPipelineSchedules {
		t.Fatalf("change = %+v, want a pipeline_schedules creation", change)
	}
	if change.Schedule.ID != 0 {
		t.Errorf("Schedule.ID = %d, want 0 for a schedule that does not exist", change.Schedule.ID)
	}
	if change.Schedule.Desired != want {
		t.Errorf("Schedule.Desired = %+v, want %+v", change.Schedule.Desired, want)
	}
}

// A schedule whose declaration and live state differ is updated, carrying the
// live id so a write can address it and both sides so a plan can say what changes.
func TestDiffPlansADifferingScheduleAsAnUpdate(t *testing.T) {
	want := declaredSchedule("nightly", "0 4 * * *")
	live := liveSchedule(7, "nightly", "0 3 * * *")

	changes := Diff(declaring([]manifest.PipelineSchedule{want}), holding([]LiveSchedule{live}))

	change := onlyScheduleChange(t, changes)
	if change.Type != ChangeUpdate {
		t.Fatalf("change = %+v, want an update", change)
	}
	if change.Schedule.ID != live.ID {
		t.Errorf("Schedule.ID = %d, want %d", change.Schedule.ID, live.ID)
	}
	if change.Schedule.Desired != want || change.Schedule.Live != toManifestSchedule(live) {
		t.Errorf("schedule = %+v, want both sides carried", change.Schedule)
	}
}

// A schedule GitLab already holds as declared is no change, so a plan over an
// unchanged project stays empty.
func TestDiffPlansNothingForAScheduleThatAlreadyMatches(t *testing.T) {
	want := declaredSchedule("nightly", "0 3 * * *")

	changes := Diff(declaring([]manifest.PipelineSchedule{want}), holding([]LiveSchedule{
		liveSchedule(7, "nightly", "0 3 * * *"),
	}))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// Renaming a schedule is a removal and a creation, and the removal comes first:
// an instance caps how many schedules a project may hold.
func TestDiffPlansARemovalBeforeTheCreationThatReplacesIt(t *testing.T) {
	changes := Diff(
		declaring([]manifest.PipelineSchedule{declaredSchedule("renamed", "0 3 * * *")}),
		holding([]LiveSchedule{liveSchedule(7, "nightly", "0 3 * * *")}),
	)

	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want a deletion and a creation", changes)
	}
	if changes[0].Type != ChangeDelete || changes[0].Schedule.Description != "nightly" {
		t.Errorf("changes[0] = %+v, want the deletion of nightly first", changes[0])
	}
	if changes[1].Type != ChangeCreate || changes[1].Schedule.Description != "renamed" {
		t.Errorf("changes[1] = %+v, want the creation of renamed second", changes[1])
	}
}

// A creation shows every attribute, because the operator is approving a whole
// schedule rather than a change to one.
func TestScheduleCreationLineShowsEveryAttribute(t *testing.T) {
	changes := Diff(
		declaring([]manifest.PipelineSchedule{declaredSchedule("nightly", "0 3 * * *")}),
		holding([]LiveSchedule{}),
	)

	got := onlyScheduleChange(t, changes).String()
	want := strings.Join([]string{
		`+ pipeline_schedule "nightly"`,
		"    + ref: refs/heads/main",
		"    + cron: 0 3 * * *",
		"    + cron_timezone: UTC",
		"    + active: true",
	}, "\n")
	if got != want {
		t.Errorf("line =\n%s\nwant\n%s", got, want)
	}
}

// An update shows only the attributes that differ, so the operator reads what
// changes rather than re-reading the schedule.
func TestScheduleUpdateLineShowsOnlyTheAttributesThatDiffer(t *testing.T) {
	changes := Diff(
		declaring([]manifest.PipelineSchedule{declaredSchedule("nightly", "0 4 * * *")}),
		holding([]LiveSchedule{liveSchedule(7, "nightly", "0 3 * * *")}),
	)

	got := onlyScheduleChange(t, changes).String()
	want := "~ pipeline_schedule \"nightly\"\n    ~ cron: 0 3 * * * → 0 4 * * *"
	if got != want {
		t.Errorf("line =\n%s\nwant\n%s", got, want)
	}
}

// A deletion names the schedule alone, because everything it holds goes with
// it.
func TestScheduleDeletionLineNamesTheScheduleAlone(t *testing.T) {
	changes := Diff(
		declaring([]manifest.PipelineSchedule{}),
		holding([]LiveSchedule{liveSchedule(7, "nightly", "0 3 * * *")}),
	)

	got := onlyScheduleChange(t, changes).String()
	if want := `- pipeline_schedule "nightly"`; got != want {
		t.Errorf("line = %q, want %q", got, want)
	}
}

// A removal names the variable keys going with the schedule, which nothing else
// in a plan mentions because the manifest holds no variables.
func TestScheduleDeletionLineNamesTheVariableKeysDestroyedWithIt(t *testing.T) {
	change := Change{Type: ChangeDelete, Field: fieldPipelineSchedules, Schedule: &ScheduleChange{
		Description:  "nightly",
		ID:           7,
		VariableKeys: []string{"DEPLOY_TOKEN", "REGION"},
	}}

	got := change.String()
	want := strings.Join([]string{
		`- pipeline_schedule "nightly"`,
		"    - variable: DEPLOY_TOKEN",
		"    - variable: REGION",
	}, "\n")
	if got != want {
		t.Errorf("line =\n%s\nwant\n%s", got, want)
	}
}
