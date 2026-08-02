package repository

import (
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

func liveVar(key, value string) ScheduleVariable {
	return ScheduleVariable{Key: key, Value: freeText(value), VariableType: "env_var"}
}

func declaredVar(key, value string) manifest.ScheduleVariable {
	return manifest.ScheduleVariable{Key: key, Value: value, VariableType: "env_var"}
}

func nightlyWithLiveVars(variables ...ScheduleVariable) LiveSchedule {
	schedule := liveNightly()
	schedule.Variables = variables
	return schedule
}

func nightlyDeclaring(variables []manifest.ScheduleVariable) manifest.PipelineSchedule {
	schedule := declaredNightly()
	schedule.Variables = variables
	return schedule
}

// An omitted block leaves the live variables alone, the way an omitted schedule
// block leaves the live schedules alone.
func TestDiffLeavesVariablesAloneWhenUnmanaged(t *testing.T) {
	got := scheduleChanges(Diff(
		declaring([]manifest.PipelineSchedule{declaredNightly()}),
		withLive(nightlyWithLiveVars(liveVar("DEPLOY_ENV", "staging"))),
	))
	if len(got) != 0 {
		t.Errorf("changes = %+v, want none for an unmanaged variable block", got)
	}
}

// The schedule's own attributes match here, so drift in the variables alone has
// to be enough to report an update.
func TestDiffUpdatesWhenOnlyTheVariablesDiffer(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		declared []manifest.ScheduleVariable
		live     []ScheduleVariable
	}{
		{
			name:     "added",
			declared: []manifest.ScheduleVariable{declaredVar("DEPLOY_ENV", "staging")},
			live:     []ScheduleVariable{},
		},
		{
			name:     "removed",
			declared: []manifest.ScheduleVariable{},
			live:     []ScheduleVariable{liveVar("DEPLOY_ENV", "staging")},
		},
		{
			name:     "value changed",
			declared: []manifest.ScheduleVariable{declaredVar("DEPLOY_ENV", "production")},
			live:     []ScheduleVariable{liveVar("DEPLOY_ENV", "staging")},
		},
		{
			name: "type changed",
			declared: []manifest.ScheduleVariable{
				{Key: "DEPLOY_ENV", Value: "staging", VariableType: "file"},
			},
			live: []ScheduleVariable{liveVar("DEPLOY_ENV", "staging")},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := scheduleChanges(Diff(
				declaring([]manifest.PipelineSchedule{nightlyDeclaring(testCase.declared)}),
				withLive(nightlyWithLiveVars(testCase.live...)),
			))
			if len(got) != 1 || got[0].Type != ChangeUpdate {
				t.Errorf("changes = %+v, want one update", got)
			}
		})
	}
}

// Matching variables are not drift, whatever order GitLab reports them in.
func TestDiffReportsNoChangeForMatchingVariables(t *testing.T) {
	declared := []manifest.ScheduleVariable{declaredVar("APP", "a"), declaredVar("ZONE", "b")}
	live := []ScheduleVariable{liveVar("ZONE", "b"), liveVar("APP", "a")}

	got := scheduleChanges(Diff(
		declaring([]manifest.PipelineSchedule{nightlyDeclaring(declared)}),
		withLive(nightlyWithLiveVars(live...)),
	))
	if len(got) != 0 {
		t.Errorf("changes = %+v, want none", got)
	}
}

// The plan output can reach a CI job log, which is read more widely than the
// repository the values live in, so it names the keys and what happens to them
// and stops there.
func TestChangeStringNamesVariableKeysWithoutTheirValues(t *testing.T) {
	change := Change{
		Type: ChangeUpdate, Name: "group/proj", Field: fieldPipelineSchedule,
		OldValue: nightlyWithLiveVars(liveVar("DEPLOY_ENV", "staging"), liveVar("OLD_FLAG", "yes")),
		NewValue: nightlyDeclaring([]manifest.ScheduleVariable{
			declaredVar("DEPLOY_ENV", "production"),
			declaredVar("API_TOKEN", "glpat-secret"),
		}),
	}

	rendered := change.String()
	for _, want := range []string{"variables:", "~ DEPLOY_ENV", "+ API_TOKEN", "- OLD_FLAG"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered block missing %q\n%s", want, rendered)
		}
	}
	for _, leaked := range []string{"staging", "production", "glpat-secret", "yes"} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("rendered block leaks the value %q\n%s", leaked, rendered)
		}
	}
}
