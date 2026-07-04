package nyctereutes

import (
	"context"
	"errors"
	"net/url"
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
	exit, _, _ := runDep(&fakeInfraGlab{}, "infra", "plan")
	if exit != 1 {
		t.Errorf("exit = %d, want 1 when no path is given", exit)
	}
}

func TestInfraPlanShowsChanges(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, _ := runDep(runner, "infra", "plan", path)

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

	exit, stdout, _ := runDep(runner, "infra", "plan", path)

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

	exit, stdout, _ := runDep(runner, "infra", "plan", path)

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

	exit, stdout, stderr := runDep(runner, "infra", "plan", path)

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

	exit, stdout, stderr := runDep(runner, "infra", "plan", path)

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

func TestInfraPlanCIExitCode(t *testing.T) {
	dir := t.TempDir()
	drift := writeManifest(t, dir, "drift.yaml", planManifest)
	match := writeManifest(t, dir, "match.yaml", matchingManifest)
	runner := &fakeInfraGlab{projects: map[string]string{targetGroupProj: projJSON}}

	t.Run("drift exits non-zero", func(t *testing.T) {
		exit, _, _ := runDep(runner, "infra", "plan", "--ci", drift)
		if exit != 1 {
			t.Errorf("exit = %d, want 1 with --ci and drift", exit)
		}
	})

	t.Run("no drift exits zero", func(t *testing.T) {
		exit, _, _ := runDep(runner, "infra", "plan", "--ci", match)
		if exit != 0 {
			t.Errorf("exit = %d, want 0 with --ci and no drift", exit)
		}
	})
}
