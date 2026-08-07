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

// planScheduleManifest declares one schedule and nothing else, so a plan
// against projJSON's live state reports the schedule alone.
const planScheduleManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  pipeline_schedules:
  - description: nightly
    ref: main
    cron: 0 3 * * *
`

// planScheduleErrGlab answers every project read from projJSON and refuses
// every schedule read, standing in for a token that may read a project but not
// the schedules it owns.
type planScheduleErrGlab struct{}

func (f *planScheduleErrGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if _, ok := catalogRead(args); ok {
		return catalogBody(false), nil
	}
	if _, ok := scheduleRead(args); ok {
		return nil, errors.New("403 Forbidden")
	}
	return []byte(projJSON), nil
}

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

// A document carrying no pipeline_schedules key makes no schedule request. The
// fake answers no schedule read, so one would surface as an unexpected call
// rather than being absorbed; that is what keeps such a project at one request.
func TestInfraPlanReadsNoSchedulesForADocumentDeclaringNone(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d (stderr %q), want 0", exit, stderr)
	}
	if !strings.Contains(stdout, "visibility") {
		t.Errorf("stdout missing the project drift\n%s", stdout)
	}
}

// A declared schedule the project does not hold is planned for creation, named
// by its description and listing what it would be created with.
func TestInfraPlanShowsAScheduleCreation(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planScheduleManifest)
	runner := &fakeInfraGlab{
		projects:  map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d (stderr %q), want 0", exit, stderr)
	}
	for _, want := range []string{`pipeline_schedule "nightly"`, "cron: 0 3 * * *", "ref: refs/heads/main"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

// A live schedule the document does not declare is planned for removal.
func TestInfraPlanShowsAScheduleRemoval(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest+"  pipeline_schedules: []\n")
	runner := &fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: `[{"id":7,"description":"nightly",` +
			`"ref":"refs/heads/main","cron":"0 3 * * *","cron_timezone":"UTC","active":true}]`},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "plan", path)

	if exit != 0 {
		t.Fatalf("exit = %d (stderr %q), want 0", exit, stderr)
	}
	if want := `- pipeline_schedule "nightly"`; !strings.Contains(stdout, want) {
		t.Errorf("stdout missing %q\n%s", want, stdout)
	}
}

// A schedule read the token may not make is reported and counted, and the rest
// of the project is still planned: ending the plan there would cost every
// setting that could be compared over one child resource.
func TestInfraPlanStillPlansAProjectWhoseSchedulesCannotBeRead(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest+"  pipeline_schedules: []\n")

	exit, stdout, stderr := runWithRunner(&planScheduleErrGlab{}, "infra", "plan", path)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 for a read that failed", exit)
	}
	if !strings.Contains(stderr, "pipeline schedules") {
		t.Errorf("stderr does not report the schedule read\n%s", stderr)
	}
	if !strings.Contains(stdout, "visibility") {
		t.Errorf("stdout does not still carry the project drift\n%s", stdout)
	}
}
