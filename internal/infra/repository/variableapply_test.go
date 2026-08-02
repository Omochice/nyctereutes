package repository

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// joinedCalls renders the recorded invocations so a test can assert on the
// sequence without repeating the argument shape.
func joinedCalls(writer *recordingWriter) []string {
	calls := make([]string, 0, len(writer.calls))
	for _, call := range writer.calls {
		calls = append(calls, strings.Join(call.args, " "))
	}
	return calls
}

// A schedule's variables live behind their own endpoints, so reconciling them
// is a call per key rather than part of the schedule's PUT.
func TestApplyReconcilesScheduleVariables(t *testing.T) {
	writer := &recordingWriter{}
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: nightlyWithLiveVars(liveVar("DEPLOY_ENV", "staging"), liveVar("OLD_FLAG", "yes")),
		NewValue: nightlyDeclaring([]manifest.ScheduleVariable{
			declaredVar("API_TOKEN", "t"),
			declaredVar("DEPLOY_ENV", "production"),
		}),
	}

	results := NewApplier(writer).Apply(context.Background(), []Change{change})
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one success", results)
	}

	calls := joinedCalls(writer)
	const base = "projects/group%2Fproj/pipeline_schedules/1/variables"
	for _, want := range []string{
		"api " + base + " --method POST -f key=API_TOKEN -f value=t",
		"api " + base + "/DEPLOY_ENV --method PUT -f value=production",
		"api " + base + "/OLD_FLAG --method DELETE",
	} {
		if !slices.ContainsFunc(calls, func(call string) bool { return strings.HasPrefix(call, want) }) {
			t.Errorf("calls = %v, want one starting %q", calls, want)
		}
	}
}

// The schedule's own attributes match here, so nothing needs writing to the
// schedule itself and a PUT would be a request that changes nothing.
func TestApplySkipsTheSchedulePutWhenOnlyVariablesDiffer(t *testing.T) {
	writer := &recordingWriter{}
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: nightlyWithLiveVars(),
		NewValue: nightlyDeclaring([]manifest.ScheduleVariable{declaredVar("APP", "a")}),
	}

	NewApplier(writer).Apply(context.Background(), []Change{change})

	for _, call := range joinedCalls(writer) {
		if strings.HasSuffix(call, "/pipeline_schedules/1 --method PUT") {
			t.Errorf("calls = %v, want no PUT on the schedule itself", joinedCalls(writer))
		}
	}
}

// A created schedule has no id until GitLab answers, so its variables are added
// against the id the create returned.
func TestApplyAddsVariablesToACreatedSchedule(t *testing.T) {
	writer := &recordingWriter{respAt: map[int][]byte{0: []byte(`{"id":42,"description":"weekly"}`)}}
	change := Change{
		Type: ChangeCreate, Name: "group/proj", Field: fieldPipelineSchedule,
		NewValue: manifest.PipelineSchedule{
			Description: "weekly", Ref: "refs/heads/main", Cron: "0 9 * * 1",
			CronTimezone: "UTC", Active: true,
			Variables: []manifest.ScheduleVariable{declaredVar("APP", "a")},
		},
	}

	results := NewApplier(writer).Apply(context.Background(), []Change{change})
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one success", results)
	}

	calls := joinedCalls(writer)
	want := "api projects/group%2Fproj/pipeline_schedules/42/variables --method POST -f key=APP -f value=a"
	if !slices.ContainsFunc(calls, func(call string) bool { return strings.HasPrefix(call, want) }) {
		t.Errorf("calls = %v, want one starting %q", calls, want)
	}
}
