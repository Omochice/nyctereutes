package nyctereutes

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// fakeApplyGlab serves project reads from a map and records every write call so
// an apply test can assert what was sent. It implements RunWithStdin as well as
// Run, so it satisfies the ProjectWriter the apply command needs. A path listed
// in failWrites makes its write fail.
type fakeApplyGlab struct {
	projects   map[string]string // "owner/name" -> project JSON, absent means 404
	writes     []string          // joined args of each write call, in order
	writeBody  []string          // parallel to writes: stdin body, "" for none
	failWrites map[string]bool   // "owner/name" whose writes fail
}

func (f *fakeApplyGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if isProjectRead(args) {
		return f.readProject(args)
	}
	return f.recordWrite(nil, args)
}

func (f *fakeApplyGlab) RunWithStdin(_ context.Context, body []byte, args ...string) ([]byte, error) {
	return f.recordWrite(body, args)
}

// A project read is the two-arg `api projects/<enc>` fetch the importer issues.
func isProjectRead(args []string) bool {
	return len(args) == 2 && args[0] == "api" && strings.HasPrefix(args[1], "projects/")
}

func (f *fakeApplyGlab) readProject(args []string) ([]byte, error) {
	path, err := url.PathUnescape(strings.TrimPrefix(args[1], "projects/"))
	if err != nil {
		return nil, fmt.Errorf("decode glab path: %w", err)
	}
	if body, ok := f.projects[path]; ok {
		return []byte(body), nil
	}
	return nil, errInfra404
}

func (f *fakeApplyGlab) recordWrite(body []byte, args []string) ([]byte, error) {
	f.writes = append(f.writes, strings.Join(args, " "))
	f.writeBody = append(f.writeBody, string(body))
	for project := range f.failWrites {
		if strings.Contains(args[1], url.PathEscape(project)) {
			return nil, errInfra404
		}
	}
	return nil, nil
}

func TestInfraApplyRequiresPath(t *testing.T) {
	exit, _, _ := runDep(&fakeApplyGlab{}, "infra", "apply")
	if exit != 1 {
		t.Errorf("exit = %d, want 1 when no path is given", exit)
	}
}

func TestInfraApplyAutoApproveAppliesChanges(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeApplyGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, _ := runDep(runner, "infra", "apply", "--auto-approve", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 for an approved apply", exit)
	}
	if len(runner.writes) != 1 {
		t.Fatalf("writes = %v, want exactly one PUT", runner.writes)
	}
	if want := "-f visibility=internal"; !strings.Contains(runner.writes[0], want) {
		t.Errorf("write %q missing %q", runner.writes[0], want)
	}
	if !strings.Contains(stdout, "visibility") {
		t.Errorf("stdout should show the plan before applying\n%s", stdout)
	}
}
