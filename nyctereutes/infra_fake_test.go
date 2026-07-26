package nyctereutes

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Omochice/nyctereutes/internal/glab"
)

var (
	// The glab runner wraps a classified not-found response with
	// glab.ErrNotFound; the fake mirrors that so FetchRepository detects the
	// 404 through errors.Is rather than the error text.
	errInfra404       = fmt.Errorf("%w: glab api: exit status 1\n404 Project Not Found", glab.ErrNotFound)
	errUnexpectedGlab = errors.New("unexpected glab call")
)

// catalogRead reports whether args is the GraphQL isCatalogResource query
// FetchRepository issues after the REST fetch, returning the fullPath it
// targets. The fakes answer it so a project read completes both calls.
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

// catalogBody is the GraphQL response body for a project's catalog status.
func catalogBody(isResource bool) []byte {
	return fmt.Appendf(nil, `{"data":{"project":{"isCatalogResource":%t}}}`, isResource)
}

// fakeInfraGlab answers `glab api projects/<enc>` from a project map and the
// catalog GraphQL query from a catalog map; an absent project yields a 404
// error so the importer treats it as missing. Any other glab invocation is an
// error so unexpected calls fail the test loudly.
type fakeInfraGlab struct {
	projects map[string]string // "owner/name" -> project JSON
	catalog  map[string]bool   // "owner/name" -> catalog status, default false
}

func (f *fakeInfraGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if path, ok := catalogRead(args); ok {
		return catalogBody(f.catalog[path]), nil
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

const targetGroupProj = "group/proj"

const projJSON = `{"description":"a tool","visibility":"private","topics":["go"],"archived":false}`
