package infra_test

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

// planFetchErrGlab answers only for okPath; every other project fails with a
// non-404 error, standing in for a network or auth failure.
type planFetchErrGlab struct {
	okPath string
	okBody string
}

func (f *planFetchErrGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	// Only okPath's REST fetch succeeds, so the catalog query is reached for
	// okPath alone; a plain not-a-resource answer keeps that project healthy.
	if _, ok := catalogRead(args); ok {
		return catalogBody(false), nil
	}
	if _, ok := scheduleRead(args); ok {
		return []byte("[]"), nil
	}
	path, err := url.PathUnescape(strings.TrimPrefix(args[1], "projects/"))
	if err != nil {
		return nil, err
	}
	if path == f.okPath {
		return []byte(f.okBody), nil
	}
	return nil, errors.New("500 Internal Server Error")
}

// planManifest declares a project whose visibility differs from projJSON's
// (private), so a plan against that live state must report the drift.
const planManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: internal
`

func TestInfraPlanRequiresPath(t *testing.T) {
	exit, _, _ := runWithRunner(&fakeInfraGlab{}, "infra", "plan")
	if exit != 1 {
		t.Errorf("exit = %d, want 1 when no path is given", exit)
	}
}

func TestInfraPlanShowsChanges(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, _ := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 for a plan with drift", exit)
	}
	for _, want := range []string{"group/proj", "visibility", "private", "internal"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

// A manifest whose declared fields all match the live project drifts in
// nothing, so the plan says so explicitly instead of printing an empty diff.
const matchingManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: private
  topics: [go]
  archived: false
`

func TestInfraPlanReportsNoChanges(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", matchingManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, _ := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "No changes.") {
		t.Errorf("stdout missing 'No changes.'\n%s", stdout)
	}
}

// A manifest for a project GitLab does not have plans as a whole-project
// create, driven by the 404 that FetchRepository turns into IsNew.
func TestInfraPlanShowsCreate(t *testing.T) {
	create := `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: fresh
  owner: group
spec:
  visibility: private
`
	path := writeManifest(t, t.TempDir(), "a.yaml", create)
	runner := &fakeInfraGlab{projects: map[string]string{}}

	exit, stdout, _ := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	for _, want := range []string{"group/fresh", "new repository"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

// A fetch failure for one project is reported and skipped without hiding the
// drift of the projects around it, and the run fails overall.
func TestInfraPlanContinuesPastFetchError(t *testing.T) {
	brokenDoc := `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: broken
  owner: group
spec:
  visibility: private
`
	stream := brokenDoc + "---\n" + planManifest
	path := writeManifest(t, t.TempDir(), "a.yaml", stream)
	runner := &planFetchErrGlab{okPath: targetGroupProj, okBody: projJSON}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a fetch fails", exit)
	}
	if !strings.Contains(stderr, "group/broken") {
		t.Errorf("stderr missing the failing project\n%s", stderr)
	}
	if !strings.Contains(stdout, "group/proj") {
		t.Errorf("stdout missing the healthy project's drift, later repos must still be planned\n%s", stdout)
	}
}

// An unparseable document is reported with its file and position and the run
// fails, but the valid documents around it are still planned.
func TestInfraPlanContinuesPastParseError(t *testing.T) {
	badDoc := strings.ReplaceAll(planManifest, "kind: Repository", "kind: Nonsense")
	stream := badDoc + "---\n" + planManifest
	path := writeManifest(t, t.TempDir(), "a.yaml", stream)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a document is invalid", exit)
	}
	for _, want := range []string{"a.yaml", "document 1"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q\n%s", want, stderr)
		}
	}
	if !strings.Contains(stdout, "group/proj") {
		t.Errorf("stdout missing the valid document's drift\n%s", stdout)
	}
}

// A directory argument plans every .yaml/.yml manifest it holds, sharing the
// non-recursive expansion the validate command uses.
func TestInfraPlanWalksDirectory(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "a.yaml", planManifest)
	writeManifest(t, dir, "b.yml", strings.ReplaceAll(planManifest, "name: proj", "name: other"))
	runner := &fakeInfraGlab{projects: map[string]string{
		"group/proj":  projJSON,
		"group/other": projJSON,
	}}

	exit, stdout, _ := runWithRunner(runner, "infra", "plan", dir)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	for _, want := range []string{"group/proj", "group/other"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

// A path that does not resolve is reported and skipped so the remaining paths
// are still planned, matching the validate command's aggregation.
func TestInfraPlanContinuesPastMissingPath(t *testing.T) {
	dir := t.TempDir()
	good := writeManifest(t, dir, "a.yaml", planManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", filepath.Join(dir, "nope.yaml"), good)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a path does not exist", exit)
	}
	if !strings.Contains(stderr, "nope.yaml") {
		t.Errorf("stderr missing the unresolved path\n%s", stderr)
	}
	if !strings.Contains(stdout, "group/proj") {
		t.Errorf("stdout missing the good path's drift, later paths must still be planned\n%s", stdout)
	}
}

func TestInfraPlanCIExitCode(t *testing.T) {
	dir := t.TempDir()
	drift := writeManifest(t, dir, "drift.yaml", planManifest)
	match := writeManifest(t, dir, "match.yaml", matchingManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	t.Run("drift exits non-zero", func(t *testing.T) {
		exit, _, _ := runWithRunner(runner, "infra", "plan", "--ci", drift)
		if exit != 1 {
			t.Errorf("exit = %d, want 1 with --ci and drift", exit)
		}
	})

	t.Run("no drift exits zero", func(t *testing.T) {
		exit, _, _ := runWithRunner(runner, "infra", "plan", "--ci", match)
		if exit != 0 {
			t.Errorf("exit = %d, want 0 with --ci and no drift", exit)
		}
	})
}

// scheduleManifest declares one schedule and the visibility projJSON already
// differs on, so a plan reports both a scalar change and schedule drift.
const scheduleManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: internal
  pipeline_schedules:
    - description: weekly
      ref: main
      cron: "0 9 * * 1"
`

// A declared list is the complete desired set, so a plan shows the schedule to
// add and the one to remove alongside the scalar drift.
func TestInfraPlanShowsScheduleDrift(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", scheduleManifest)
	runner := &fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: `[
		  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *",
		   "cron_timezone":"UTC","active":true}]`},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", "--ci", path)

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 when --ci sees drift\n%s", exit, stderr)
	}
	for _, want := range []string{
		`- pipeline_schedule "nightly"`,
		`+ pipeline_schedule "weekly"`,
		"cron:",
		"0 9 * * 1",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

// The declared schedule carries an empty variables block so the run reads the
// live variables: without it the read is skipped and a value could not leak
// whatever the renderer did, which would leave this test proving nothing.
const scheduleWithVariablesManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: internal
  pipeline_schedules:
    - description: weekly
      ref: main
      cron: "0 9 * * 1"
      variables: []
`

// The schema now expresses a schedule's variables, so removing one is an
// ordinary deletion rather than something to refuse, and the plan names the
// keys that go with it. The value stays out: a plan is read aloud and pasted
// into issues, and a schedule variable can hold a credential.
func TestInfraPlanDeletesAScheduleWithVariables(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", scheduleWithVariablesManifest)
	runner := &fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: `[
		  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *",
		   "cron_timezone":"UTC","active":true}]`},
		variables: map[string]string{
			"1": `{"variables":[{"variable_type":"env_var","key":"DEPLOY_ENV","value":"staging","raw":false}]}`,
		},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", exit, stderr)
	}
	if !strings.Contains(stdout, `- pipeline_schedule "nightly"`) {
		t.Errorf("stdout missing the deletion\n%s", stdout)
	}
	if strings.Contains(stdout, "staging") {
		t.Errorf("stdout leaks a variable value\n%s", stdout)
	}
}

// countingInfraGlab records how many schedule reads a run performs so a test
// can assert that a manifest managing no schedule pays for none.
type countingInfraGlab struct {
	inner fakeInfraGlab

	listCalls   int
	singleCalls int
}

func (f *countingInfraGlab) Run(ctx context.Context, args ...string) ([]byte, error) {
	if _, ok := scheduleRead(args); ok {
		f.listCalls++
	}
	if _, ok := singleScheduleRead(args); ok {
		f.singleCalls++
	}
	return f.inner.Run(ctx, args...)
}

// A manifest that declares no pipeline_schedules manages none, so a project
// whose schedules are unreadable or ambiguous must still plan: a schedule
// problem cannot be allowed to hide the drift of every other setting.
func TestInfraPlanIgnoresSchedulesWhenTheManifestDeclaresNone(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &countingInfraGlab{inner: fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		// Two schedules share a description, which no manifest could describe.
		schedules: map[string]string{targetGroupProj: `[
		  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *"},
		  {"id":5,"description":"nightly","ref":"refs/heads/main","cron":"0 8 * * *"}]`},
	}}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", exit, stderr)
	}
	if !strings.Contains(stdout, "visibility") {
		t.Errorf("stdout missing the unrelated drift\n%s", stdout)
	}
	if runner.listCalls != 0 {
		t.Errorf("schedule list calls = %d, want 0 when no schedule is managed", runner.listCalls)
	}
}

// The variables cost a request per schedule, so a manifest that declares
// schedules but no variables must not pay for them.
func TestInfraPlanReadsNoVariablesWhenNoneAreDeclared(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", scheduleManifest)
	runner := &countingInfraGlab{inner: fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: `[
		  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *"}]`},
	}}

	exit, _, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", exit, stderr)
	}
	if runner.listCalls != 1 {
		t.Errorf("schedule list calls = %d, want 1", runner.listCalls)
	}
	if runner.singleCalls != 0 {
		t.Errorf("variable reads = %d, want 0 when no variable is declared", runner.singleCalls)
	}
}

// A manifest that manages variables against a token that cannot read them
// would plan every declared variable as an addition and every live one as
// already gone, because GitLab answers such a reader with the field left out.
// The token cannot write them either, so the run stops and says which schedule
// it could not read.
const variablesDeclaredManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: private
  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      variables:
        - key: DEPLOY_ENV
          value: staging
`

func TestInfraPlanRefusesVariablesItCannotRead(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", variablesDeclaredManifest)
	runner := &fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: `[{"id":1,"description":"nightly",
		  "ref":"refs/heads/main","cron":"0 3 * * *","cron_timezone":"UTC","active":true}]`},
		// The single-schedule body GitLab returns to a reader who may not see
		// the variables: the field is absent rather than empty.
		variables: map[string]string{"1": `{"id":1,"description":"nightly"}`},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit == 0 {
		t.Errorf("exit = 0, want non-zero when the variables cannot be read")
	}
	if !strings.Contains(stderr, "nightly") {
		t.Errorf("stderr should name the schedule that could not be read\n%s", stderr)
	}
	if strings.Contains(stdout, "DEPLOY_ENV") {
		t.Errorf("no variable change may be planned against a state never read\n%s", stdout)
	}
}
