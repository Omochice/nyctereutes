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
