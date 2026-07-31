package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
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

// isScheduleCall reports whether args address the pipeline schedules endpoint,
// so a runner can tell that read apart from the project fetch.
func isScheduleCall(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return strings.Contains(arg, "pipeline_schedules")
	})
}

// scheduleRunner answers the calls a fetch makes: the GraphQL catalog query,
// the schedule list, one variables read per schedule, and the project itself.
// Schedules answer with no variables; a test about variables builds its own.
func scheduleRunner(listJSON string, record *[]string) glab.RunnerFunc {
	return func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) > 1 && args[1] == "graphql":
			return []byte(graphqlCatalogJSON(false)), nil
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

// GitLab accepts two schedules described alike on one project, but a manifest
// pairs a declared schedule with a live one by description, so such a project
// cannot be described at all. Reporting it names both ids because renaming one
// of them in the UI is what the reader has to do.
func TestFetchRepositoryRejectsDuplicateLiveDescriptions(t *testing.T) {
	const duplicateJSON = `[
	  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *","cron_timezone":"UTC","active":true},
	  {"id":5,"description":"nightly","ref":"refs/heads/main","cron":"0 8 * * *","cron_timezone":"UTC","active":true}
	]`

	_, err := NewClient(scheduleRunner(duplicateJSON, nil)).
		FetchSchedules(context.Background(), ownerGroup, nameProj)
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

// A schedule read can fail or be ambiguous on its own terms, so folding it into
// the project read would let a schedule problem hide every other setting the
// project drifts in. A project read therefore asks for no schedule at all.
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

// GitLab lists schedules in id order, which is creation order, so a schedule
// deleted and recreated moves. A manifest is a file under version control, so
// the same live state has to produce the same bytes; ordering by description,
// which the schema already forces to be unique, does that.
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

// The list endpoint pages at 20 while an instance can raise the schedule limit
// above that, so the read asks glab to follow every page rather than silently
// describing a project by its first page alone.
func TestFetchRepositoryListsSchedulesAcrossPages(t *testing.T) {
	var args []string
	_, err := NewClient(scheduleRunner(scheduleListJSON, &args)).
		FetchSchedules(context.Background(), "group/sub", "proj")
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

func TestFetchRepositoryParsesSchedules(t *testing.T) {
	schedules, err := NewClient(scheduleRunner(scheduleListJSON, nil)).
		FetchSchedules(context.Background(), "group", "proj")
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
