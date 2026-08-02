package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

func scheduleCreate(desired manifest.PipelineSchedule) Change {
	return Change{
		Type: ChangeCreate, Name: "group/proj", Field: fieldPipelineSchedule, NewValue: desired,
	}
}

func scheduleDelete(live LiveSchedule) Change {
	return Change{
		Type: ChangeDelete, Name: "group/proj", Field: fieldPipelineSchedule, OldValue: live,
	}
}

func scheduleUpdate(desired manifest.PipelineSchedule, live LiveSchedule) Change {
	return Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: live, NewValue: desired,
	}
}

// A schedule is created through its own endpoint rather than the projects PUT,
// and every attribute goes with it because nothing exists to merge into.
func TestApplyCreatesASchedule(t *testing.T) {
	writer := &recordingWriter{}
	desired := manifest.PipelineSchedule{
		Description: "weekly report", Ref: "refs/tags/v1.0.0",
		Cron: "0 9 * * 1", CronTimezone: "Asia/Tokyo", Active: false,
	}

	results := NewApplier(writer).Apply(context.Background(), []Change{scheduleCreate(desired)})

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one success", results)
	}
	args := writer.calls[0].args
	for _, want := range []string{
		"api", "projects/group%2Fproj/pipeline_schedules", "--method", "POST",
		"description=weekly report", "ref=refs/tags/v1.0.0", "cron=0 9 * * 1",
		"cron_timezone=Asia/Tokyo", "active=false",
	} {
		if !slices.Contains(args, want) {
			t.Errorf("args = %v, want %q", args, want)
		}
	}
}

// An update addresses the schedule by the id GitLab assigned, which the change
// carries from the live read because no manifest holds it.
func TestApplyUpdatesAScheduleByItsLiveID(t *testing.T) {
	writer := &recordingWriter{}
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: LiveSchedule{ID: 7, Description: "nightly"},
		NewValue: manifest.PipelineSchedule{
			Description: "nightly", Ref: "refs/heads/main",
			Cron: "0 4 * * *", CronTimezone: "UTC", Active: true,
		},
	}

	results := NewApplier(writer).Apply(context.Background(), []Change{change})

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one success", results)
	}
	args := writer.calls[0].args
	for _, want := range []string{
		"projects/group%2Fproj/pipeline_schedules/7", "--method", "PUT",
		"cron=0 4 * * *", "ref=refs/heads/main", "active=true",
	} {
		if !slices.Contains(args, want) {
			t.Errorf("args = %v, want %q", args, want)
		}
	}
}

func TestApplyDeletesAScheduleByItsLiveID(t *testing.T) {
	writer := &recordingWriter{}

	results := NewApplier(writer).Apply(context.Background(),
		[]Change{scheduleDelete(LiveSchedule{ID: 5, Description: "old"})})

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one success", results)
	}
	args := writer.calls[0].args
	for _, want := range []string{"projects/group%2Fproj/pipeline_schedules/5", "--method", "DELETE"} {
		if !slices.Contains(args, want) {
			t.Errorf("args = %v, want %q", args, want)
		}
	}
}

// A create can be relying on a delete for its slot under the per-project
// schedule limit, so a failed delete strands it. It is reported rather than
// dropped, so the report accounts for every change the plan showed.
func TestApplySkipsAScheduleCreateAfterAFailedDelete(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}
	changes := []Change{
		scheduleDelete(LiveSchedule{ID: 5, Description: "old"}),
		scheduleCreate(manifest.PipelineSchedule{Description: "weekly", Ref: "refs/heads/main", Cron: "0 9 * * 1"}),
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 2 {
		t.Fatalf("results = %+v, want one per planned change", results)
	}
	if results[0].Err == nil {
		t.Error("first result should carry the failure")
	}
	if results[1].Err == nil || !errors.Is(results[1].Err, errScheduleChangeSkipped) {
		t.Errorf("second result = %v, want it reported as skipped", results[1].Err)
	}
	if len(writer.calls) != 1 {
		t.Errorf("calls = %d, want the create not to have run", len(writer.calls))
	}
}

// The scalar settings do not depend on the schedules, so a failed schedule
// change must not hold them back.
func TestApplyStillAppliesScalarChangesAfterAScheduleFailure(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}
	changes := []Change{
		scheduleDelete(LiveSchedule{ID: 5, Description: "old"}),
		{Type: ChangeUpdate, Name: "group/proj", Field: fieldDescription, NewValue: "a tool"},
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 2 || results[1].Err != nil {
		t.Fatalf("results = %+v, want the scalar change applied", results)
	}
	if len(writer.calls) != 2 {
		t.Errorf("calls = %d, want the scalar PUT to have run", len(writer.calls))
	}
}

// A maintainer who does not own a schedule cannot update it, which the generic
// permission hint misreports as a missing role the token already holds.
func TestApplyReportsScheduleOwnershipOnForbidden(t *testing.T) {
	forbidden := fmt.Errorf("%w: glab api: exit status 1\n403 Forbidden", glab.ErrForbidden)
	writer := &recordingWriter{errAt: map[int]error{0: forbidden}}
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: LiveSchedule{ID: 7, Description: "nightly", Cron: "0 3 * * *"},
		NewValue: manifest.PipelineSchedule{Description: "nightly", Cron: "0 4 * * *"},
	}

	results := NewApplier(writer).Apply(context.Background(), []Change{change})

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results = %+v, want one failure", results)
	}
	if !strings.Contains(results[0].Err.Error(), "owned by another user") {
		t.Errorf("error = %v, want it to explain schedule ownership", results[0].Err)
	}
}

// Only a failed delete strands the schedule changes after it, because a delete
// is what makes room under the per-project limit. A failed create or update
// leaves the rest applicable, so stopping there would strand work for nothing.
func TestApplyContinuesAfterAFailedScheduleCreate(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}
	changes := []Change{
		scheduleCreate(manifest.PipelineSchedule{Description: "weekly", Ref: "refs/heads/main", Cron: "0 9 * * 1"}),
		scheduleCreate(manifest.PipelineSchedule{Description: "monthly", Ref: "refs/heads/main", Cron: "0 9 1 * *"}),
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 2 {
		t.Fatalf("results = %+v, want one per change", results)
	}
	if results[0].Err == nil {
		t.Error("first result should carry the failure")
	}
	if results[1].Err != nil {
		t.Errorf("second result = %v, want the independent create attempted", results[1].Err)
	}
	if len(writer.calls) != 2 {
		t.Errorf("calls = %d, want both creates attempted", len(writer.calls))
	}
}

// A key is a path segment, so it is percent-encoded like every other one the
// package builds; interpolating it raw would address a different variable.
func TestApplyEncodesVariableKeysInThePath(t *testing.T) {
	writer := &recordingWriter{}
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: nightlyWithLiveVars(liveVar("A B", "x")),
		NewValue: nightlyDeclaring([]manifest.ScheduleVariable{}),
	}

	NewApplier(writer).Apply(context.Background(), []Change{change})

	if len(writer.calls) != 1 {
		t.Fatalf("calls = %v, want one delete", writer.calls)
	}
	if want := "/variables/A%20B"; !strings.Contains(strings.Join(writer.calls[0].args, " "), want) {
		t.Errorf("args = %v, want the key encoded as %q", writer.calls[0].args, want)
	}
}

// Only a create needs the slot a delete frees. An update addresses a schedule
// that already exists, and a later delete frees a slot rather than taking one,
// so neither is abandoned because an earlier delete failed.
func TestApplyKeepsGoingPastAFailedDeleteForChangesNeedingNoSlot(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}
	changes := []Change{
		scheduleDelete(LiveSchedule{ID: 5, Description: "old"}),
		scheduleDelete(LiveSchedule{ID: 6, Description: "older"}),
		scheduleUpdate(
			manifest.PipelineSchedule{Description: "nightly", Ref: "refs/heads/main", Cron: "0 4 * * *"},
			LiveSchedule{ID: 7, Description: "nightly", Ref: "refs/heads/main", Cron: "0 3 * * *"},
		),
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 3 {
		t.Fatalf("results = %+v, want one per planned change", results)
	}
	if results[0].Err == nil {
		t.Error("first result should carry the failure")
	}
	for _, index := range []int{1, 2} {
		if errors.Is(results[index].Err, errScheduleChangeSkipped) {
			t.Errorf("result %d was skipped, want it attempted: %v", index, results[index].Err)
		}
	}
	if len(writer.calls) != 3 {
		t.Errorf("calls = %d, want the later delete and the update to have run", len(writer.calls))
	}
}
