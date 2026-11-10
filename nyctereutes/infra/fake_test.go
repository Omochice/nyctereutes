package infra_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/nyctereutes"
)

var (
	// The glab runner wraps a classified not-found response with
	// glab.ErrNotFound; the fake mirrors that so FetchRepository detects the
	// 404 through errors.Is rather than the error text.
	errInfra404       = fmt.Errorf("%w: glab api: exit status 1\n404 Project Not Found", glab.ErrNotFound)
	errUnexpectedGlab = errors.New("unexpected glab call")
)

// Reports whether args is the GraphQL isCatalogResource query FetchRepository
// issues after the REST fetch, returning the fullPath it targets. The fakes
// answer it so a project read completes both calls.
func catalogRead(args []string) (fullPath string, ok bool) {
	if len(args) < 2 || args[0] != "api" || args[1] != "graphql" {
		return "", false
	}
	// Require the query name, not just fullPath, to tell the read from other calls.
	if !strings.Contains(strings.Join(args, " "), "isCatalogResource") {
		return "", false
	}
	for _, a := range args {
		if p, found := strings.CutPrefix(a, "fullPath="); found {
			return p, true
		}
	}
	return "", false
}

// The GraphQL response body for a project's catalog status.
func catalogBody(isResource bool) []byte {
	return fmt.Appendf(nil, `{"data":{"project":{"isCatalogResource":%t}}}`, isResource)
}

// Both child-collection reads differ only in the endpoint suffix, so one matcher serves them.
func collectionRead(args []string, suffix string) (project string, ok bool) {
	if len(args) != 3 || args[0] != "api" || args[1] != "--paginate" {
		return "", false
	}
	endpoint, found := strings.CutPrefix(args[2], "projects/")
	if !found {
		return "", false
	}
	encoded, found := strings.CutSuffix(endpoint, suffix)
	if !found {
		return "", false
	}
	path, err := url.PathUnescape(encoded)
	if err != nil {
		return "", false
	}
	return path, true
}

// A project absent from the map owns none of the resource, which is the answer GitLab gives for most of them.
func collectionBody(bodies map[string]string, project string) []byte {
	if body, ok := bodies[project]; ok {
		return []byte(body)
	}
	return []byte("[]")
}

// Reports whether args is the single-schedule read the disclosure of destroyed
// variables issues, returning the schedule id it addresses. It has the same
// two-argument shape as a project read, so a fake must try this one first or
// answer a disclosure with a project.
func scheduleDetailRead(args []string) (id string, ok bool) {
	if len(args) != 2 || args[0] != "api" {
		return "", false
	}
	endpoint, found := strings.CutPrefix(args[1], "projects/")
	if !found {
		return "", false
	}
	_, id, found = strings.Cut(endpoint, "/pipeline_schedules/")
	return id, found
}

// The single-schedule response for a disclosure read, carrying the variables a
// test scripted for that schedule id. A schedule absent from the map holds
// none, which is what most of them hold. An empty entry leaves the field out of
// the response altogether, which is how GitLab answers a reader who may not see
// them.
func scheduleDetailBody(variables map[string]string, id string) []byte {
	field, ok := variables[id]
	if !ok {
		field = "[]"
	}
	if field == "" {
		return []byte(`{"description":"nightly"}`)
	}
	return fmt.Appendf(nil, `{"variables":%s}`, field)
}

// fakeInfraGlab answers `glab api projects/<enc>` from a project map and the catalog GraphQL query from a catalog map.
// An absent project yields a 404 error so the importer treats it as missing.
// Any other glab invocation is an error so unexpected calls fail the test loudly.
//
// A nil child-collection map turns that read into an unexpected call, which is how a test asserts none is made.
type fakeInfraGlab struct {
	projects  map[string]string // "owner/name" -> project JSON
	catalog   map[string]bool   // "owner/name" -> catalog status, default false
	schedules map[string]string // "owner/name" -> schedule list JSON; nil forbids the read
	branches  map[string]string // "owner/name" -> protected branch list JSON; nil forbids the read
	variables map[string]string // schedule id -> the variables field; absent holds none
}

func (f *fakeInfraGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if path, ok := catalogRead(args); ok {
		return catalogBody(f.catalog[path]), nil
	}
	if body, ok := f.collection(args); ok {
		return body, nil
	}
	if id, ok := scheduleDetailRead(args); ok {
		return scheduleDetailBody(f.variables, id), nil
	}
	return f.project(args)
}

func (f *fakeInfraGlab) collection(args []string) ([]byte, bool) {
	for _, collection := range []struct {
		suffix string
		bodies map[string]string
	}{
		{suffix: "/pipeline_schedules", bodies: f.schedules},
		{suffix: "/protected_branches", bodies: f.branches},
	} {
		if collection.bodies == nil {
			continue
		}
		if path, ok := collectionRead(args, collection.suffix); ok {
			return collectionBody(collection.bodies, path), true
		}
	}
	return nil, false
}

func (f *fakeInfraGlab) project(args []string) ([]byte, error) {
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

// The empty maps declare both child reads expected while leaving every project owning none.
func importFake(projects map[string]string) *fakeInfraGlab {
	return &fakeInfraGlab{
		projects:  projects,
		schedules: map[string]string{},
		branches:  map[string]string{},
	}
}

// Drives the whole command tree with an injected glab runner, which is how the
// infra subcommands are exercised end to end: the exit code and the diagnostics
// these tests assert on are produced by the dispatcher, not by Execute.
func runWithRunner(runner glab.Runner, args ...string) (exit int, stdout, stderr string) {
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	exit = nyctereutes.Dispatch(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	}, runner)
	return exit, outBuf.String(), errBuf.String()
}

const targetGroupProj = "group/proj"

const projJSON = `{"description":"a tool","visibility":"private","topics":["go"],"archived":false}`
