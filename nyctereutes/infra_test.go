package nyctereutes

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

var (
	errInfra404       = errors.New("glab api: exit status 1\n404 Project Not Found")
	errUnexpectedGlab = errors.New("unexpected glab call")
)

// fakeInfraGlab answers `glab api projects/<enc>` from a project map; an absent
// project yields a 404 error so the importer treats it as missing. Any other
// glab invocation is an error so unexpected calls fail the test loudly.
type fakeInfraGlab struct {
	projects map[string]string // "owner/name" -> project JSON
}

func (f *fakeInfraGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if len(args) != 2 || args[0] != "api" || !strings.HasPrefix(args[1], "projects/") {
		return nil, fmt.Errorf("%w: %v", errUnexpectedGlab, args)
	}
	path, err := url.PathUnescape(strings.TrimPrefix(args[1], "projects/"))
	if err != nil {
		return nil, fmt.Errorf("decode glab path: %w", err)
	}
	if body, ok := f.projects[path]; ok {
		return []byte(body), nil
	}
	return nil, errInfra404
}

const projJSON = `{"description":"a tool","visibility":"private","topics":["go"],"archived":false}`

func TestInfraImportEmitsYAML(t *testing.T) {
	runner := &fakeInfraGlab{projects: map[string]string{"group/proj": projJSON}}
	exit, stdout, _ := runDep(runner, "infra", "import", "group/proj")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	for _, want := range []string{"kind: Repository", "name: proj", "owner: group", "a tool"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

func TestInfraImportKeepsEmptyTopics(t *testing.T) {
	noTopics := `{"description":"d","visibility":"private","topics":[],"archived":false}`
	runner := &fakeInfraGlab{projects: map[string]string{"group/proj": noTopics}}
	exit, stdout, _ := runDep(runner, "infra", "import", "group/proj")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "topics: []") {
		t.Errorf("an empty topic list should be exported as 'topics: []'\n%s", stdout)
	}
}

func TestInfraImportSeparatesMultipleDocs(t *testing.T) {
	runner := &fakeInfraGlab{projects: map[string]string{
		"group/a": projJSON,
		"group/b": projJSON,
	}}
	exit, stdout, _ := runDep(runner, "infra", "import", "group/a", "group/b")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "\n---\n") {
		t.Errorf("multiple docs should be separated by ---\n%s", stdout)
	}
}

func TestInfraImportRequiresTarget(t *testing.T) {
	exit, _, _ := runDep(&fakeInfraGlab{}, "infra", "import")
	if exit != 1 {
		t.Errorf("exit = %d, want 1 when no project is given", exit)
	}
}

func TestInfraImportContinuesPastMissing(t *testing.T) {
	runner := &fakeInfraGlab{projects: map[string]string{"group/ok": projJSON}}
	exit, stdout, stderr := runDep(runner, "infra", "import", "group/missing", "group/ok")

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a project is missing", exit)
	}
	if !strings.Contains(stdout, "name: ok") {
		t.Errorf("the existing project should still be exported\n%s", stdout)
	}
	if !strings.Contains(stderr, "missing") {
		t.Errorf("the missing project should be reported on stderr\n%s", stderr)
	}
}
