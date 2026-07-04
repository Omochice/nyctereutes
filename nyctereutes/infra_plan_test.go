package nyctereutes

import (
	"strings"
	"testing"
)

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
