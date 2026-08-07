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

// Reports whether args is the paginated pipeline schedule list FetchSchedules
// issues, returning the project it targets. Only import calls it, and it is the
// only schedule request any command makes; the fake uses this to tell that read
// apart from the project fetch it must still guard.
func scheduleRead(args []string) (project string, ok bool) {
	if len(args) != 3 || args[0] != "api" || args[1] != "--paginate" {
		return "", false
	}
	endpoint, found := strings.CutPrefix(args[2], "projects/")
	if !found {
		return "", false
	}
	encoded, found := strings.CutSuffix(endpoint, "/pipeline_schedules")
	if !found {
		return "", false
	}
	path, err := url.PathUnescape(encoded)
	if err != nil {
		return "", false
	}
	return path, true
}

// The list response for a project. A project absent from the map owns no
// schedule, which is the answer GitLab gives for most of them.
func scheduleBody(schedules map[string]string, project string) []byte {
	if body, ok := schedules[project]; ok {
		return []byte(body)
	}
	return []byte("[]")
}

// fakeInfraGlab answers `glab api projects/<enc>` from a project map and the
// catalog GraphQL query from a catalog map; an absent project yields a 404
// error so the importer treats it as missing. Any other glab invocation is an
// error so unexpected calls fail the test loudly.
//
// A nil schedules map keeps a schedule read inside that guard, so only a test
// that declares the command reads schedules can make one. Answering it for
// every command would let plan or apply grow a schedule read unnoticed, which
// is the read FetchRepository is documented to keep out of them.
type fakeInfraGlab struct {
	projects  map[string]string // "owner/name" -> project JSON
	catalog   map[string]bool   // "owner/name" -> catalog status, default false
	schedules map[string]string // "owner/name" -> schedule list JSON; nil forbids the read
}

func (f *fakeInfraGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if path, ok := catalogRead(args); ok {
		return catalogBody(f.catalog[path]), nil
	}
	if path, ok := scheduleRead(args); ok && f.schedules != nil {
		return scheduleBody(f.schedules, path), nil
	}
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

// A fake for the import command, whose export reads schedules. The empty map
// declares that read expected while leaving every project owning none, which is
// what the tests that are not about schedules want.
func importFake(projects map[string]string) *fakeInfraGlab {
	return &fakeInfraGlab{projects: projects, schedules: map[string]string{}}
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
