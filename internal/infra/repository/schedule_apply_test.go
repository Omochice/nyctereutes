package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// One schedule change addressed to group/proj.
func scheduleWrite(kind ChangeType, schedule ScheduleChange) []Change {
	return []Change{{
		Type: kind, Name: "group/proj", Field: fieldPipelineSchedules, Schedule: &schedule,
	}}
}

// The one glab invocation the applier made, failing the test if it made more.
func onlyCall(t *testing.T, writer *recordingWriter) string {
	t.Helper()
	if len(writer.calls) != 1 {
		t.Fatalf("calls = %v, want exactly 1", writer.calls)
	}
	return strings.Join(writer.calls[0].args, " ")
}

// A creation posts to the collection with every attribute, because none of them
// exists yet.
func TestApplyCreatesAScheduleThroughTheCollectionEndpoint(t *testing.T) {
	writer := &recordingWriter{}
	changes := scheduleWrite(ChangeCreate, ScheduleChange{
		Description: "nightly",
		Desired:     declaredSchedule("nightly", "0 3 * * *"),
	})

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one successful result", results)
	}
	want := "api projects/group%2Fproj/pipeline_schedules --method POST " +
		"-f description=nightly -f ref=refs/heads/main -f cron=0 3 * * * " +
		"-f cron_timezone=UTC -f active=true"
	if got := onlyCall(t, writer); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// An update puts to the schedule's own endpoint, addressed by the id the read
// reported, and sends every attribute because GitLab changes only what it is
// given.
func TestApplyUpdatesAScheduleByItsID(t *testing.T) {
	writer := &recordingWriter{}
	changes := scheduleWrite(ChangeUpdate, ScheduleChange{
		Description: "nightly",
		ID:          7,
		Desired:     declaredSchedule("nightly", "0 4 * * *"),
		Live:        declaredSchedule("nightly", "0 3 * * *"),
	})

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one successful result", results)
	}
	want := "api projects/group%2Fproj/pipeline_schedules/7 --method PUT " +
		"-f description=nightly -f ref=refs/heads/main -f cron=0 4 * * * " +
		"-f cron_timezone=UTC -f active=true"
	if got := onlyCall(t, writer); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// A paused schedule is written as active=false rather than being left out,
// which would let GitLab's create default turn it back on.
func TestApplySendsAPausedScheduleAsInactive(t *testing.T) {
	writer := &recordingWriter{}
	paused := declaredSchedule("nightly", "0 3 * * *")
	paused.Active = false

	NewApplier(writer).Apply(context.Background(), scheduleWrite(ChangeCreate, ScheduleChange{
		Description: "nightly", Desired: paused,
	}))

	if got := onlyCall(t, writer); !strings.Contains(got, "-f active=false") {
		t.Errorf("glab args = %q, want active=false", got)
	}
}

// A removal deletes the schedule's own endpoint and sends no attributes.
func TestApplyDeletesAScheduleByItsID(t *testing.T) {
	writer := &recordingWriter{}
	changes := scheduleWrite(ChangeDelete, ScheduleChange{
		Description: "nightly", ID: 7, Live: declaredSchedule("nightly", "0 3 * * *"),
	})

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one successful result", results)
	}
	want := "api projects/group%2Fproj/pipeline_schedules/7 --method DELETE"
	if got := onlyCall(t, writer); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// A refused write names the schedule as the plan named it.
func TestApplyScheduleFailureNamesTheSchedule(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: glab.ErrForbidden}}
	changes := scheduleWrite(ChangeDelete, ScheduleChange{Description: "nightly", ID: 7})

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results = %+v, want one failed result", results)
	}
	for _, want := range []string{
		`pipeline_schedule "nightly"`, "group/proj", "permission denied", "created by this token's user",
	} {
		if !strings.Contains(results[0].Err.Error(), want) {
			t.Errorf("error %q does not name %q", results[0].Err, want)
		}
	}
}

// A schedule change carrying no schedule is reported rather than addressed to
// pipeline_schedules/0, which is what a nil would otherwise be written as.
func TestApplyRefusesAScheduleChangeCarryingNoSchedule(t *testing.T) {
	writer := &recordingWriter{}
	changes := []Change{{Type: ChangeDelete, Name: "group/proj", Field: fieldPipelineSchedules}}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || !errors.Is(results[0].Err, errUnexpectedValueType) {
		t.Fatalf("results = %+v, want one refused result", results)
	}
	if len(writer.calls) != 0 {
		t.Errorf("calls = %v, want none", writer.calls)
	}
}
