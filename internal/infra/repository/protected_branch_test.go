package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Three protections as GitLab CE reports them, captured from 19.2.1.
// The automatic protection of the default branch, a branch nobody may push to, and a wildcard.
// Each carries the deploy_key_id GitLab always writes and nulls.
const protectedBranchListJSON = `[
  {"id":1,"name":"main",
   "push_access_levels":[{"id":1,"access_level":40,"access_level_description":"Maintainers","deploy_key_id":null}],
   "merge_access_levels":[{"id":1,"access_level":40,"access_level_description":"Maintainers"}],
   "allow_force_push":false},
  {"id":3,"name":"release/1.x",
   "push_access_levels":[{"id":3,"access_level":0,"access_level_description":"No one","deploy_key_id":null}],
   "merge_access_levels":[{"id":3,"access_level":30,"access_level_description":"Developers + Maintainers"}],
   "allow_force_push":true},
  {"id":4,"name":"feature/*",
   "push_access_levels":[{"id":4,"access_level":60,"access_level_description":"Instance admins","deploy_key_id":null}],
   "merge_access_levels":[{"id":4,"access_level":40,"access_level_description":"Maintainers"}],
   "allow_force_push":false}
]`

// Reports whether args address the protected branches endpoint.
// A runner uses it to tell that read apart from the project fetch.
func isProtectedBranchCall(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return strings.Contains(arg, "protected_branches")
	})
}

// Answers the calls a fetch makes: the GraphQL catalog query, the branch list from listJSON, and the project itself.
// The args of the branch call go to record when one is given, which is what the test about paging asserts on.
func protectedBranchRunner(listJSON string, record *[]string) glab.RunnerFunc {
	return func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) > 1 && args[1] == "graphql":
			return []byte(graphqlCatalogJSON(false)), nil
		case isProtectedBranchCall(args):
			if record != nil {
				*record = args
			}
			return []byte(listJSON), nil
		default:
			return []byte(sampleProjectJSON), nil
		}
	}
}

// Reads the branches or fails the test, which is what every assertion below about a well-formed list wants.
func fetchBranches(t *testing.T, listJSON string) []LiveProtectedBranch {
	t.Helper()
	branches, err := NewClient(protectedBranchRunner(listJSON, nil)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)
	if err != nil {
		t.Fatalf("fetch protected branches: %v", err)
	}
	return branches
}

// The read follows the pages.
// GitLab pages this endpoint the way it pages the schedules.
// A project holding more protections than one page carries would otherwise be described by its first page alone.
func TestFetchProtectedBranchesFollowsThePages(t *testing.T) {
	var called []string
	_, err := NewClient(protectedBranchRunner(protectedBranchListJSON, &called)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)
	if err != nil {
		t.Fatalf("fetch protected branches: %v", err)
	}
	if !slices.Contains(called, "--paginate") {
		t.Errorf("call = %v, want it to follow the pages", called)
	}
}

// Each role GitLab reports as a number becomes the name a manifest declares.
// The branches come out ordered by name, so the same live state emits the same bytes.
func TestProtectedBranchesAreDescribedByNameAndRole(t *testing.T) {
	got, err := toManifestProtectedBranches(fetchBranches(t, protectedBranchListJSON))
	if err != nil {
		t.Fatalf("describe protected branches: %v", err)
	}
	want := []manifest.ProtectedBranch{
		{Name: "feature/*", PushAccessLevel: "admin", MergeAccessLevel: "maintainer"},
		{Name: "main", PushAccessLevel: "maintainer", MergeAccessLevel: "maintainer"},
		{Name: "release/1.x", PushAccessLevel: "no_one", MergeAccessLevel: "developer", AllowForcePush: true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("branches = %+v, want %+v", got, want)
	}
}

// A project that was never read for its branches says nothing about them.
// The manifest field keeps that apart from a project declared to hold none.
func TestUnreadProtectedBranchesAreNotDescribed(t *testing.T) {
	got, err := toManifestProtectedBranches(nil)
	if err != nil {
		t.Fatalf("describe protected branches: %v", err)
	}
	if got != nil {
		t.Errorf("branches = %v, want nil (unread)", got)
	}
}

// A grant the manifest cannot hold is refused rather than flattened to the role beside it.
// Describing such a branch and applying the document back would revoke the grant that was dropped.
// A deploy key is the shape a free-tier project actually reaches, so it is the one that must not slip through.
func TestFetchProtectedBranchesRejectsGrantsNoManifestCanHold(t *testing.T) {
	for _, testCase := range []struct {
		name, listJSON, wantNamed string
	}{
		{
			name: "deploy key",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[
			    {"id":1,"access_level":40,"deploy_key_id":null},
			    {"id":2,"access_level":40,"deploy_key_id":7}],
			  "merge_access_levels":[{"id":1,"access_level":40}]}]`,
			wantNamed: "push_access_levels",
		},
		{
			name: "user",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[{"id":1,"access_level":40,"user_id":9}],
			  "merge_access_levels":[{"id":1,"access_level":40}]}]`,
			wantNamed: "user_id",
		},
		{
			name: "group",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[{"id":1,"access_level":40}],
			  "merge_access_levels":[{"id":1,"access_level":40,"group_id":3}]}]`,
			wantNamed: "group_id",
		},
		{
			name: "inherited from a group",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[{"id":1,"access_level":40,"inherited":true}],
			  "merge_access_levels":[{"id":1,"access_level":40}]}]`,
			wantNamed: "inherited",
		},
		{
			name: "no grant at all",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[],
			  "merge_access_levels":[{"id":1,"access_level":40}]}]`,
			wantNamed: "push_access_levels",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewClient(protectedBranchRunner(testCase.listJSON, nil)).
				FetchProtectedBranches(context.Background(), ownerGroup, nameProj)
			if !errors.Is(err, errIndescribableBranch) {
				t.Fatalf("err = %v, want an indescribable branch", err)
			}
			if !strings.Contains(err.Error(), "main") {
				t.Errorf("error = %q, want it to name the branch", err)
			}
			if !strings.Contains(err.Error(), testCase.wantNamed) {
				t.Errorf("error = %q, want it to name %q", err, testCase.wantNamed)
			}
		})
	}
}

// A role GitLab reports that no manifest names is reported rather than read as one of the roles it is not.
// A level a later GitLab adds therefore cannot be exported as "developer" and then applied as one.
func TestDescribingAProtectedBranchRejectsAnUnknownRole(t *testing.T) {
	const unknownRoleJSON = `[{"id":1,"name":"main",
	  "push_access_levels":[{"id":1,"access_level":25}],
	  "merge_access_levels":[{"id":1,"access_level":40}]}]`

	_, err := toManifestProtectedBranches(fetchBranches(t, unknownRoleJSON))

	if !errors.Is(err, errUnknownBranchAccessLevel) {
		t.Fatalf("err = %v, want an unknown access level", err)
	}
	if !strings.Contains(err.Error(), "25") {
		t.Errorf("error = %q, want it to name the level it could not read", err)
	}
}

// A response carrying no page is refused rather than read as a project holding no protection.
// The manifest field would carry that into a document as a declaration that none may exist.
// Applying such a document would unprotect everything.
func TestFetchProtectedBranchesRejectsAResponseWithNoPage(t *testing.T) {
	_, err := NewClient(protectedBranchRunner("null", nil)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)

	if !errors.Is(err, errNoProtectedBranchPage) {
		t.Errorf("err = %v, want the empty-response sentinel", err)
	}
}
