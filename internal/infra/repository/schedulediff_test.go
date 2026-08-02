package repository

import (
	"reflect"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// liveNightly is the live counterpart of declaredNightly, matching it in every
// attribute so a test can vary one at a time.
func liveNightly() LiveSchedule {
	return LiveSchedule{
		ID: 1, Description: "nightly", Ref: "refs/heads/main",
		Cron: "0 3 * * *", CronTimezone: "UTC", Active: true,
	}
}

func declaredNightly() manifest.PipelineSchedule {
	return manifest.PipelineSchedule{
		Description: "nightly", Ref: "refs/heads/main",
		Cron: "0 3 * * *", CronTimezone: "UTC", Active: true,
	}
}

// declaring builds a manifest whose schedule block is exactly schedules, so a
// nil argument declares nothing and an empty slice declares none.
func declaring(schedules []manifest.PipelineSchedule) *manifest.Repository {
	return &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: ownerGroup, Name: nameProj},
		Spec:     manifest.RepositorySpec{PipelineSchedules: schedules},
	}
}

func withLive(schedules ...LiveSchedule) *CurrentState {
	return &CurrentState{Owner: ownerGroup, Name: nameProj, PipelineSchedules: schedules}
}

// scheduleChanges keeps only the schedule changes, so a test is not disturbed
// by the scalar settings the same diff reports.
func scheduleChanges(changes []Change) []Change {
	var kept []Change
	for _, change := range changes {
		if change.Field == fieldPipelineSchedule {
			kept = append(kept, change)
		}
	}
	return kept
}

// An omitted block leaves the live schedules alone, the way an omitted scalar
// setting leaves its value alone.
func TestDiffLeavesSchedulesAloneWhenUnmanaged(t *testing.T) {
	got := scheduleChanges(Diff(declaring(nil), withLive(liveNightly())))
	if len(got) != 0 {
		t.Errorf("changes = %+v, want none for an unmanaged block", got)
	}
}

// A declared list is the complete desired set, so a schedule GitLab does not
// have is created and one the list does not name is removed.
func TestDiffCreatesAndDeletesToMatchTheDeclaredSet(t *testing.T) {
	declared := manifest.PipelineSchedule{
		Description: "weekly", Ref: "refs/heads/main", Cron: "0 9 * * 1", CronTimezone: "UTC", Active: true,
	}

	got := scheduleChanges(Diff(declaring([]manifest.PipelineSchedule{declared}), withLive(liveNightly())))
	if len(got) != 2 {
		t.Fatalf("changes = %+v, want a delete and a create", got)
	}

	// Deletes come first so a replacement cannot hit the per-project limit.
	if got[0].Type != ChangeDelete {
		t.Errorf("first change = %v, want a delete", got[0].Type)
	}
	if live, ok := got[0].OldValue.(LiveSchedule); !ok || live.ID != 1 {
		t.Errorf("delete OldValue = %+v, want the live schedule with its id", got[0].OldValue)
	}
	if got[1].Type != ChangeCreate {
		t.Errorf("second change = %v, want a create", got[1].Type)
	}
	if want, ok := got[1].NewValue.(manifest.PipelineSchedule); !ok || want.Description != "weekly" {
		t.Errorf("create NewValue = %+v, want the declared schedule", got[1].NewValue)
	}
}

// An explicit empty list declares that the project should own no schedule.
func TestDiffDeletesEveryScheduleForAnEmptyList(t *testing.T) {
	got := scheduleChanges(Diff(declaring([]manifest.PipelineSchedule{}), withLive(liveNightly())))
	if len(got) != 1 || got[0].Type != ChangeDelete {
		t.Errorf("changes = %+v, want one delete", got)
	}
}

func TestDiffReportsNoChangeForAMatchingSchedule(t *testing.T) {
	got := scheduleChanges(Diff(
		declaring([]manifest.PipelineSchedule{declaredNightly()}), withLive(liveNightly()),
	))
	if len(got) != 0 {
		t.Errorf("changes = %+v, want none", got)
	}
}

// Every attribute the manifest carries is compared, so drift in any of them is
// reported rather than a subset being silently accepted.
func TestDiffUpdatesEachChangedScheduleAttribute(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*manifest.PipelineSchedule)
	}{
		{"cron", func(s *manifest.PipelineSchedule) { s.Cron = "0 4 * * *" }},
		{"ref", func(s *manifest.PipelineSchedule) { s.Ref = "refs/tags/v1.0.0" }},
		{"cron_timezone", func(s *manifest.PipelineSchedule) { s.CronTimezone = "Asia/Tokyo" }},
		{"active", func(s *manifest.PipelineSchedule) { s.Active = false }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			declared := declaredNightly()
			testCase.mutate(&declared)

			got := scheduleChanges(Diff(
				declaring([]manifest.PipelineSchedule{declared}), withLive(liveNightly()),
			))
			if len(got) != 1 || got[0].Type != ChangeUpdate {
				t.Fatalf("changes = %+v, want one update", got)
			}
			if live, ok := got[0].OldValue.(LiveSchedule); !ok || live.ID != 1 {
				t.Errorf("update OldValue = %+v, want the live schedule with its id", got[0].OldValue)
			}
			if want, ok := got[0].NewValue.(manifest.PipelineSchedule); !ok || !reflect.DeepEqual(want, declared) {
				t.Errorf("update NewValue = %+v, want the declared schedule", got[0].NewValue)
			}
		})
	}
}

// The description is the identity, so changing it in the manifest reads as
// removing one schedule and adding another rather than as a rename.
func TestDiffTreatsARenameAsADeleteAndACreate(t *testing.T) {
	renamed := declaredNightly()
	renamed.Description = "nightly-2"

	got := scheduleChanges(Diff(declaring([]manifest.PipelineSchedule{renamed}), withLive(liveNightly())))
	if len(got) != 2 || got[0].Type != ChangeDelete || got[1].Type != ChangeCreate {
		t.Errorf("changes = %+v, want a delete followed by a create", got)
	}
}
