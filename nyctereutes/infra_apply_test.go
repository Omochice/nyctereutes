package nyctereutes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
)

// fakeApplyGlab serves project reads from a map and records every write call so
// an apply test can assert what was sent. It implements RunWithStdin as well as
// Run, so it satisfies the ProjectWriter the apply command needs. A path listed
// in failWrites makes its write fail.
type fakeApplyGlab struct {
	projects   map[string]string // "owner/name" -> project JSON, absent means 404
	catalog    map[string]bool   // "owner/name" -> catalog status, default false
	writes     []string          // joined args of each write call, in order
	writeBody  []string          // parallel to writes: stdin body, "" for none
	failWrites map[string]bool   // "owner/name" whose writes fail
	fetchFail  map[string]bool   // "owner/name" whose read fails with a non-404 error
}

func (f *fakeApplyGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if isProjectRead(args) {
		return f.readProject(args)
	}
	// The catalog query is part of the read, not a write, so it must not be
	// recorded as one.
	if path, ok := catalogRead(args); ok {
		return catalogBody(f.catalog[path]), nil
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
	if f.fetchFail[path] {
		return nil, errFetch500
	}
	if body, ok := f.projects[path]; ok {
		return []byte(body), nil
	}
	return nil, errInfra404
}

// A non-404 read failure, standing in for a network or auth error.
var errFetch500 = errors.New("500 Internal Server Error")

// runInfraApply drives dispatch with the given stdin so a confirmation prompt
// can be answered.
func runInfraApply(stdin string, runner glab.Runner, args ...string) (exit int, stdout, stderr string) {
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	exit = dispatch(args, &cli.ProcInout{
		Stdin:  strings.NewReader(stdin),
		Stdout: outBuf,
		Stderr: errBuf,
	}, runner)
	return exit, outBuf.String(), errBuf.String()
}

func (f *fakeApplyGlab) recordWrite(body []byte, args []string) ([]byte, error) {
	joined := strings.Join(args, " ")
	f.writes = append(f.writes, joined)
	f.writeBody = append(f.writeBody, string(body))
	for project := range f.failWrites {
		// A REST write carries the project URL-escaped in the endpoint (args[1]),
		// while a catalog mutation carries it raw in a projectPath form arg, so
		// both encodings are matched to fail either write path.
		if strings.Contains(args[1], url.PathEscape(project)) ||
			strings.Contains(joined, "projectPath="+project) {
			return nil, errInfra404
		}
	}
	// A catalog mutation's success is read from its payload, so it must answer
	// with the realistic GraphQL shape (an empty errors array under the mutation
	// field). A bare "{}" would decode to the same success while hiding a
	// payload-parsing regression, so the real shape is returned instead.
	if field, ok := catalogMutationField(args); ok {
		return fmt.Appendf(nil, `{"data":{"%s":{"errors":[]}}}`, field), nil
	}
	return nil, nil
}

// catalogMutationField reports which catalog mutation a GraphQL write invokes,
// so the fake can answer with a payload keyed by that mutation field.
func catalogMutationField(args []string) (string, bool) {
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "graphql") {
		return "", false
	}
	for _, field := range []string{"catalogResourcesCreate", "catalogResourcesDestroy"} {
		if strings.Contains(joined, field) {
			return field, true
		}
	}
	return "", false
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

// catalogManifest asks to publish the project to the CI/CD Catalog; projJSON's
// live state is not a catalog resource, so applying it must run the GraphQL
// mutation rather than a REST PUT.
const catalogManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  description: a tool
  ci_catalog: true
`

// A ci_catalog change is applied through the catalogResourcesCreate GraphQL
// mutation, not the projects REST endpoint, so the end-to-end apply must issue
// the mutation addressing the project by its raw projectPath.
func TestInfraApplyPublishesCatalogThroughMutation(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", catalogManifest)
	runner := &fakeApplyGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, _, _ := runDep(runner, "infra", "apply", "--auto-approve", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 for an approved apply", exit)
	}
	if len(runner.writes) != 1 {
		t.Fatalf("writes = %v, want exactly one mutation", runner.writes)
	}
	for _, want := range []string{"api graphql", "catalogResourcesCreate", "projectPath=group/proj"} {
		if !strings.Contains(runner.writes[0], want) {
			t.Errorf("write %q missing %q", runner.writes[0], want)
		}
	}
}

// noDriftManifest declares only the visibility projJSON already has, so a plan
// against that live state reports nothing.
const noDriftManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: private
`

// goneManifest targets a project the fake does not have, so its fetch 404s and
// the diff yields a create.
const goneManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: gone
  owner: group
spec:
  visibility: private
`

func TestInfraApplyPromptAppliesOnYes(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeApplyGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, _ := runInfraApply("y\n", runner, "infra", "apply", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(runner.writes) != 1 {
		t.Fatalf("writes = %v, want one PUT after a yes", runner.writes)
	}
	if !strings.Contains(stdout, "Apply these changes?") {
		t.Errorf("stdout should carry the prompt\n%s", stdout)
	}
}

func TestInfraApplyPromptCancels(t *testing.T) {
	for _, answer := range []string{"n\n", ""} {
		runner := &fakeApplyGlab{projects: map[string]string{targetGroupProj: projJSON}}
		path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)

		exit, stdout, _ := runInfraApply(answer, runner, "infra", "apply", path)

		if exit != 0 {
			t.Errorf("answer %q: exit = %d, want 0 on cancel", answer, exit)
		}
		if len(runner.writes) != 0 {
			t.Errorf("answer %q: writes = %v, want none on cancel", answer, runner.writes)
		}
		if !strings.Contains(stdout, "Apply canceled.") {
			t.Errorf("answer %q: stdout should say canceled\n%s", answer, stdout)
		}
	}
}

func TestInfraApplyNoChanges(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", noDriftManifest)
	runner := &fakeApplyGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, stdout, _ := runInfraApply("", runner, "infra", "apply", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "No changes.") {
		t.Errorf("stdout should report no changes\n%s", stdout)
	}
	if len(runner.writes) != 0 {
		t.Errorf("writes = %v, want none", runner.writes)
	}
	if strings.Contains(stdout, "Apply these changes?") {
		t.Errorf("must not prompt when there is nothing to apply\n%s", stdout)
	}
}

func TestInfraApplyReportsMissingProjectButAppliesRest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "gone.yaml", goneManifest)
	writeManifest(t, dir, "proj.yaml", planManifest)
	runner := &fakeApplyGlab{projects: map[string]string{targetGroupProj: projJSON}}

	exit, _, stderr := runInfraApply("", runner, "infra", "apply", "--auto-approve", dir)

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 when a project is missing", exit)
	}
	if !strings.Contains(stderr, "group/gone") {
		t.Errorf("stderr should name the missing project\n%s", stderr)
	}
	if len(runner.writes) != 1 {
		t.Errorf("writes = %v, want the existing project still applied", runner.writes)
	}
}

func TestInfraApplyReportsWriteFailure(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeApplyGlab{
		projects:   map[string]string{targetGroupProj: projJSON},
		failWrites: map[string]bool{targetGroupProj: true},
	}

	exit, _, stderr := runInfraApply("", runner, "infra", "apply", "--auto-approve", path)

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 when a write fails", exit)
	}
	if !strings.Contains(stderr, "visibility") {
		t.Errorf("stderr should name the failed field\n%s", stderr)
	}
}

func TestInfraApplyReportsFetchError(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", planManifest)
	runner := &fakeApplyGlab{
		projects:  map[string]string{targetGroupProj: projJSON},
		fetchFail: map[string]bool{targetGroupProj: true},
	}

	exit, _, stderr := runInfraApply("", runner, "infra", "apply", "--auto-approve", path)

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 on a fetch error", exit)
	}
	if len(runner.writes) != 0 {
		t.Errorf("writes = %v, want none when the fetch failed", runner.writes)
	}
	if !strings.Contains(stderr, "group/proj") {
		t.Errorf("stderr should name the project\n%s", stderr)
	}
}

func TestInfraApplyExpandsDirectory(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "one.yaml", planManifest)
	writeManifest(t, dir, "two.yaml", strings.ReplaceAll(planManifest, "proj", "other"))
	runner := &fakeApplyGlab{projects: map[string]string{
		targetGroupProj: projJSON,
		"group/other":   projJSON,
	}}

	exit, _, _ := runInfraApply("", runner, "infra", "apply", "--auto-approve", dir)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(runner.writes) != 2 {
		t.Errorf("writes = %v, want one per manifest in the directory", runner.writes)
	}
}
