package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Two schedules as the list endpoint reports them, trimmed to the attributes
// the import reads. The refs are full paths and one schedule is paused, the way
// GitLab answers.
const scheduleListJSON = `[
  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *",
   "cron_timezone":"UTC","active":true,"next_run_at":"2026-07-31T03:03:00.000Z"},
  {"id":2,"description":"weekly report","ref":"refs/tags/v1.0.0","cron":"0 9 * * 1",
   "cron_timezone":"Asia/Tokyo","active":false,"next_run_at":null}
]`

// Reports whether args address the pipeline schedules endpoint, so a runner can
// tell that read apart from the project fetch.
func isScheduleCall(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return strings.Contains(arg, "pipeline_schedules")
	})
}

// isSingleScheduleCall reports whether args address one schedule rather than
// the list, which is the call that carries the variables.
func isSingleScheduleCall(args []string) bool {
	_, id, found := strings.Cut(args[len(args)-1], "/pipeline_schedules/")
	return found && id != ""
}

// Answers the calls a fetch makes: the GraphQL catalog query, the schedule list
// from listJSON, and the project itself. The args of the schedule call go to
// record when one is given, which is what the tests about paging assert on.
func scheduleRunner(listJSON string, record *[]string) glab.RunnerFunc {
	return func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) > 1 && args[1] == "graphql":
			return []byte(graphqlCatalogJSON(false)), nil
		case isSingleScheduleCall(args):
			return []byte(`{"variables":[]}`), nil
		case isScheduleCall(args):
			if record != nil {
				*record = args
			}
			return []byte(listJSON), nil
		default:
			return []byte(sampleProjectJSON), nil
		}
	}
}

// variableRunner answers a fetch where each schedule has its own single-schedule
// body, keyed by id, and records which of those bodies were requested.
func variableRunner(listJSON string, single map[string]string, calls *[]string) glab.RunnerFunc {
	return func(_ context.Context, args ...string) ([]byte, error) {
		endpoint := args[len(args)-1]
		switch {
		case len(args) > 1 && args[1] == "graphql":
			return []byte(graphqlCatalogJSON(false)), nil
		case isSingleScheduleCall(args):
			_, id, _ := strings.Cut(endpoint, "/pipeline_schedules/")
			*calls = append(*calls, endpoint)
			return []byte(single[id]), nil
		case isScheduleCall(args):
			return []byte(listJSON), nil
		default:
			return []byte(sampleProjectJSON), nil
		}
	}
}

// The list response carries no variables at all, so the only way to know
// whether a schedule has any is to fetch it on its own. A later slice refuses
// to delete a schedule carrying variables the schema cannot express, which is
// what makes this read necessary before variables reach the manifest.
func TestFetchRepositoryReadsScheduleVariables(t *testing.T) {
	const listJSON = `[
	  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *","cron_timezone":"UTC","active":true},
	  {"id":2,"description":"weekly","ref":"refs/heads/main","cron":"0 9 * * 1","cron_timezone":"UTC","active":true}
	]`
	single := map[string]string{
		"1": `{"id":1,"description":"nightly","variables":[
		  {"variable_type":"env_var","key":"DEPLOY_ENV","value":"staging","raw":false},
		  {"variable_type":"file","key":"CONFIG","value":"line one\nline two","raw":false}]}`,
		"2": `{"id":2,"description":"weekly","variables":[]}`,
	}
	var singleCalls []string

	schedules, err := NewClient(variableRunner(listJSON, single, &singleCalls)).
		FetchSchedules(context.Background(), ownerGroup, nameProj, true)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}

	if len(singleCalls) != 2 {
		t.Fatalf("single-schedule calls = %v, want one per schedule", singleCalls)
	}
	nightly := schedules[0]
	if len(nightly.Variables) != 2 {
		t.Fatalf("nightly variables = %+v, want 2", nightly.Variables)
	}
	want := ScheduleVariable{Key: "DEPLOY_ENV", Value: "staging", VariableType: "env_var"}
	if nightly.Variables[0] != want {
		t.Errorf("variable = %+v, want %+v", nightly.Variables[0], want)
	}
	if got := schedules[1].Variables; len(got) != 0 {
		t.Errorf("weekly variables = %+v, want none", got)
	}
}

// A project whose live schedules repeat a description is rejected, and the
// error names the description and both ids, which is what the reader needs to
// find the pair and rename one.
func TestFetchSchedulesRejectsDuplicateDescriptions(t *testing.T) {
	const duplicateJSON = `[
	  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *","cron_timezone":"UTC","active":true},
	  {"id":5,"description":"nightly","ref":"refs/heads/main","cron":"0 8 * * *","cron_timezone":"UTC","active":true}
	]`

	_, err := NewClient(scheduleRunner(duplicateJSON, nil)).
		FetchSchedules(context.Background(), ownerGroup, nameProj, false)
	if err == nil {
		t.Fatal("FetchSchedules should reject duplicate live descriptions, got nil")
	}
	if !errors.Is(err, errDuplicateLiveSchedule) {
		t.Errorf("error = %v, want it to wrap errDuplicateLiveSchedule", err)
	}
	for _, want := range []string{"nightly", "1", "5"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// A project read asks for no schedule at all and leaves the field unset, which
// is the separation [Client.FetchRepository] documents.
func TestFetchRepositoryDoesNotReadSchedules(t *testing.T) {
	var scheduleCalls int
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case isScheduleCall(args):
			scheduleCalls++
			return []byte("[]"), nil
		case len(args) > 1 && args[1] == "graphql":
			return []byte(graphqlCatalogJSON(false)), nil
		default:
			return []byte(sampleProjectJSON), nil
		}
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), ownerGroup, nameProj)
	if err != nil {
		t.Fatalf("FetchRepository: %v", err)
	}
	if state.PipelineSchedules != nil {
		t.Errorf("schedules = %v, want nil (not read)", state.PipelineSchedules)
	}
	if scheduleCalls != 0 {
		t.Errorf("schedule calls = %d, want 0 from a project read", scheduleCalls)
	}
}

// The variables cost a request per schedule, so a caller that manages none must
// not pay for them.
func TestFetchSchedulesSkipsVariablesWhenNotAsked(t *testing.T) {
	var singleCalls int
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case isSingleScheduleCall(args):
			singleCalls++
			return []byte(`{"variables":[]}`), nil
		case isScheduleCall(args):
			return []byte(scheduleListJSON), nil
		default:
			return []byte(sampleProjectJSON), nil
		}
	})

	schedules, err := NewClient(runner).FetchSchedules(context.Background(), ownerGroup, nameProj, false)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}
	if len(schedules) != 2 {
		t.Fatalf("schedules = %+v, want 2", schedules)
	}
	if singleCalls != 0 {
		t.Errorf("single-schedule calls = %d, want 0 when variables are not asked for", singleCalls)
	}
}

// Schedules reach the manifest ordered by description rather than in the id
// order GitLab lists them in, and each one's live attributes are carried
// through unchanged.
func TestToManifestSortsSchedulesByDescription(t *testing.T) {
	state := &CurrentState{
		Owner: ownerGroup,
		Name:  nameProj,
		PipelineSchedules: []LiveSchedule{
			{ID: 1, Description: "nightly", Ref: "refs/heads/main", Cron: "0 3 * * *", CronTimezone: "UTC", Active: true},
			{ID: 2, Description: "full ref", Ref: "refs/tags/v1", Cron: "0 5 * * *", CronTimezone: "UTC", Active: true},
			{ID: 3, Description: "weekly", Ref: "refs/heads/main", Cron: "0 9 * * 1", CronTimezone: "Asia/Tokyo"},
		},
	}

	got := ToManifest(state).Spec.PipelineSchedules
	descriptions := make([]string, 0, len(got))
	for _, schedule := range got {
		descriptions = append(descriptions, schedule.Description)
	}
	want := []string{"full ref", "nightly", "weekly"}
	if !slices.Equal(descriptions, want) {
		t.Errorf("descriptions = %v, want %v", descriptions, want)
	}

	weekly := got[2]
	if string(weekly.Ref) != "refs/heads/main" || weekly.Cron != "0 9 * * 1" ||
		weekly.CronTimezone != "Asia/Tokyo" || weekly.Active {
		t.Errorf("weekly = %+v, want the live attributes carried through", weekly)
	}
}

// The read asks glab for --paginate against the encoded schedules endpoint, so
// a project is not described by its first page alone.
func TestFetchSchedulesAsksGlabToPaginate(t *testing.T) {
	var args []string
	_, err := NewClient(scheduleRunner(scheduleListJSON, &args)).
		FetchSchedules(context.Background(), "group/sub", "proj", false)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}

	if !slices.Contains(args, "--paginate") {
		t.Errorf("args = %v, want --paginate", args)
	}
	want := "projects/group%2Fsub%2Fproj/pipeline_schedules"
	if !slices.Contains(args, want) {
		t.Errorf("args = %v, want the encoded endpoint %q", args, want)
	}
}

// In --paginate mode glab writes one JSON array per page back to back, so the
// whole output is not a single JSON value. A reader that decoded once would
// keep the first page and drop the rest without reporting anything.
func TestFetchSchedulesJoinsEveryPage(t *testing.T) {
	const twoPages = `[{"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *"}]
[{"id":2,"description":"weekly","ref":"refs/heads/main","cron":"0 9 * * 1"}]`

	schedules, err := NewClient(scheduleRunner(twoPages, nil)).
		FetchSchedules(context.Background(), ownerGroup, nameProj, false)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}

	descriptions := make([]string, 0, len(schedules))
	for _, schedule := range schedules {
		descriptions = append(descriptions, schedule.Description)
	}
	if want := []string{"nightly", "weekly"}; !slices.Equal(descriptions, want) {
		t.Errorf("descriptions = %v, want %v from both pages", descriptions, want)
	}
}

// Each of the three attributes the schema requires is refused when GitLab
// reports it blank, and the error names the schedule and the attribute, which is
// what the reader needs to go and find it.
func TestFetchSchedulesRejectsAScheduleMissingARequiredAttribute(t *testing.T) {
	for _, testCase := range []struct{ field, body string }{
		{field: "description", body: `[{"id":7,"description":"","ref":"refs/heads/main","cron":"0 3 * * *"}]`},
		{field: "ref", body: `[{"id":7,"description":"nightly","ref":"","cron":"0 3 * * *"}]`},
		{field: "cron", body: `[{"id":7,"description":"nightly","ref":"refs/heads/main","cron":""}]`},
	} {
		t.Run(testCase.field, func(t *testing.T) {
			_, err := NewClient(scheduleRunner(testCase.body, nil)).
				FetchSchedules(context.Background(), ownerGroup, nameProj, false)
			if !errors.Is(err, errIncompleteLiveSchedule) {
				t.Fatalf("error = %v, want it to wrap errIncompleteLiveSchedule", err)
			}
			for _, want := range []string{"7", testCase.field} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// A schedule carrying all three reads without complaint, so the guard refuses
// only what it is meant to. The timezone is left out because the schema does not
// require it, which a fixture supplying one could not show.
func TestFetchSchedulesAcceptsAScheduleGitLabReportsInFull(t *testing.T) {
	const body = `[{"id":7,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *"}]`

	schedules, err := NewClient(scheduleRunner(body, nil)).
		FetchSchedules(context.Background(), ownerGroup, nameProj, false)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}
	if len(schedules) != 1 {
		t.Errorf("schedules = %+v, want 1", schedules)
	}
}

// A response carrying no list at all is an error rather than a read of zero
// schedules.
func TestFetchSchedulesRefusesAResponseWithoutAList(t *testing.T) {
	for name, body := range map[string]string{
		"empty":      "",
		"whitespace": "  \n",
		"null":       "null",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewClient(scheduleRunner(body, nil)).
				FetchSchedules(context.Background(), ownerGroup, nameProj, false)
			if !errors.Is(err, errNoSchedulePage) {
				t.Errorf("error = %v, want it to wrap errNoSchedulePage", err)
			}
		})
	}
}

// A project that genuinely owns no schedule answers with an empty list, which
// stays a successful read of zero schedules.
func TestFetchSchedulesReadsAnEmptyListAsNoSchedules(t *testing.T) {
	schedules, err := NewClient(scheduleRunner("[]", nil)).
		FetchSchedules(context.Background(), ownerGroup, nameProj, false)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("schedules = %+v, want none", schedules)
	}
}

func TestFetchSchedulesParsesEveryAttribute(t *testing.T) {
	schedules, err := NewClient(scheduleRunner(scheduleListJSON, nil)).
		FetchSchedules(context.Background(), "group", "proj", false)
	if err != nil {
		t.Fatalf("FetchSchedules: %v", err)
	}

	if len(schedules) != 2 {
		t.Fatalf("schedules = %+v, want 2", schedules)
	}
	got := schedules[1]
	want := LiveSchedule{
		ID:           2,
		Description:  "weekly report",
		Ref:          "refs/tags/v1.0.0",
		Cron:         "0 9 * * 1",
		CronTimezone: "Asia/Tokyo",
		Active:       false,
	}
	if got.ID != want.ID || got.Description != want.Description || got.Ref != want.Ref ||
		got.Cron != want.Cron || got.CronTimezone != want.CronTimezone || got.Active != want.Active {
		t.Errorf("schedule = %+v, want %+v", got, want)
	}
}

// A manifest is a file under version control, so the same live state has to
// produce the same bytes; GitLab returns a schedule's variables in no
// guaranteed order, and the key is unique within a schedule.
func TestToManifestSortsVariablesByKey(t *testing.T) {
	state := &CurrentState{
		Owner: ownerGroup,
		Name:  nameProj,
		PipelineSchedules: []LiveSchedule{{
			ID: 1, Description: "nightly", Ref: "refs/heads/main", Cron: "0 3 * * *",
			CronTimezone: "UTC", Active: true,
			Variables: []ScheduleVariable{
				{Key: "ZONE", Value: "b", VariableType: "env_var"},
				{Key: "APP", Value: "a", VariableType: "file"},
			},
		}, {
			ID: 2, Description: "weekly", Ref: "refs/heads/main", Cron: "0 9 * * 1",
			CronTimezone: "UTC", Active: true,
		}},
	}

	schedules := ToManifest(state).Spec.PipelineSchedules
	got := schedules[0].Variables
	if len(got) != 2 || got[0].Key != "APP" || got[1].Key != "ZONE" {
		t.Fatalf("variables = %+v, want APP before ZONE", got)
	}
	want := manifest.ScheduleVariable{Key: "APP", Value: "a", VariableType: "file"}
	if got[0] != want {
		t.Errorf("variable = %+v, want %+v", got[0], want)
	}
	// The weekly schedule's variables were never read, so the document says
	// nothing about them rather than declaring that it carries none.
	if weekly := schedules[1].Variables; weekly != nil {
		t.Errorf("weekly variables = %v, want nil (unread)", weekly)
	}
}

// GitLab answers a schedule that carries no variable with an empty list, which
// is a read rather than an absence, so the document declares the empty list.
func TestToManifestDeclaresAReadEmptyVariableList(t *testing.T) {
	state := &CurrentState{
		Owner: ownerGroup,
		Name:  nameProj,
		PipelineSchedules: []LiveSchedule{{
			ID: 1, Description: "nightly", Ref: "refs/heads/main", Cron: "0 3 * * *",
			CronTimezone: "UTC", Active: true,
			Variables: []ScheduleVariable{},
		}},
	}

	got := ToManifest(state).Spec.PipelineSchedules[0].Variables
	if got == nil || len(got) != 0 {
		t.Errorf("variables = %v, want an empty list that declares none", got)
	}
}
