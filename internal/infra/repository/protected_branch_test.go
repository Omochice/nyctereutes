package repository

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Captured from GitLab CE 19.2.1, which writes deploy_key_id as null on every plain role.
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

func isProtectedBranchCall(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return strings.Contains(arg, "protected_branches")
	})
}

type protectedBranchGlab struct {
	list       string
	enterprise bool
	versionErr error
	called     *[]string
}

func (g protectedBranchGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	switch {
	case len(args) > 1 && args[1] == "graphql":
		return []byte(graphqlCatalogJSON(false)), nil
	case len(args) == 2 && args[1] == "version":
		if g.versionErr != nil {
			return nil, g.versionErr
		}
		return []byte(`{"version":"19.5.3","enterprise":` + strconv.FormatBool(g.enterprise) + `}`), nil
	case isProtectedBranchCall(args):
		if g.called != nil {
			*g.called = args
		}
		return []byte(g.list), nil
	default:
		return []byte(sampleProjectJSON), nil
	}
}

func protectedBranchRunner(listJSON string, record *[]string) protectedBranchGlab {
	return protectedBranchGlab{list: listJSON, called: record}
}

func fetchBranches(t *testing.T, listJSON string) []manifest.ProtectedBranch {
	t.Helper()
	return fetchProtections(t, listJSON).Branches
}

func fetchProtections(t *testing.T, listJSON string) *ProtectedBranches {
	t.Helper()
	read, err := NewClient(protectedBranchRunner(listJSON, nil)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)
	if err != nil {
		t.Fatalf("fetch protected branches: %v", err)
	}
	return read
}

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

func TestProtectedBranchesAreDescribedByNameAndRole(t *testing.T) {
	got := fetchBranches(t, protectedBranchListJSON)
	want := []manifest.ProtectedBranch{
		{Name: "feature/*", PushAccessLevel: "admin", MergeAccessLevel: "maintainer"},
		{Name: "main", PushAccessLevel: "maintainer", MergeAccessLevel: "maintainer"},
		{Name: "release/1.x", PushAccessLevel: "no_one", MergeAccessLevel: "developer", AllowForcePush: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("branches = %+v, want %+v", got, want)
	}
}

// Unread is not the same answer as declared to hold none.
func TestUnreadProtectedBranchesAreNotDescribed(t *testing.T) {
	got, err := toManifestProtectedBranches(nil)
	if err != nil {
		t.Fatalf("describe protected branches: %v", err)
	}
	if got != nil {
		t.Errorf("branches = %v, want nil (unread)", got)
	}
}

// Flattened to the role beside it, the exported document would revoke the dropped grant once applied.
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

// A level a later GitLab adds must not be exported as "developer" and then applied as one.
func TestDescribingAProtectedBranchRejectsAnUnknownRole(t *testing.T) {
	const unknownRoleJSON = `[{"id":1,"name":"main",
	  "push_access_levels":[{"id":1,"access_level":25}],
	  "merge_access_levels":[{"id":1,"access_level":40}]}]`

	_, err := NewClient(protectedBranchRunner(unknownRoleJSON, nil)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)

	if !errors.Is(err, errUnknownBranchAccessLevel) {
		t.Fatalf("err = %v, want an unknown access level", err)
	}
	if !strings.Contains(err.Error(), "25") {
		t.Errorf("error = %q, want it to name the level it could not read", err)
	}
}

// Read as an empty list, the document would declare that none may exist, and applying it would unprotect everything.
func TestFetchProtectedBranchesRejectsAResponseWithNoPage(t *testing.T) {
	_, err := NewClient(protectedBranchRunner("null", nil)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)

	if !errors.Is(err, errNoProtectedBranchPage) {
		t.Errorf("err = %v, want the empty-response sentinel", err)
	}
}

// A CE build omits unprotect_access_levels and an Enterprise one reports it empty for a default protection.
func TestAnUnreportedUnprotectLevelIsNotDescribed(t *testing.T) {
	for _, testCase := range []struct{ name, listJSON string }{
		{
			name: "community edition omits the field",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[{"id":1,"access_level":40}],
			  "merge_access_levels":[{"id":1,"access_level":40}]}]`,
		},
		{
			name: "enterprise reports an empty list",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[{"id":1,"access_level":40}],
			  "merge_access_levels":[{"id":1,"access_level":40}],
			  "unprotect_access_levels":[]}]`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			branches := fetchBranches(t, testCase.listJSON)
			if level := branches[0].UnprotectAccessLevel; level != nil {
				t.Errorf("unprotect level = %v, want nil (nothing to describe)", *level)
			}
		})
	}
}

// The manifest holds the role so that reconciling the branch does not change it.
func TestAnEnterpriseUnprotectLevelIsDescribed(t *testing.T) {
	const eeJSON = `[{"id":1,"name":"main",
	  "push_access_levels":[{"id":1,"access_level":40,"user_id":null,"group_id":null}],
	  "merge_access_levels":[{"id":1,"access_level":40,"user_id":null,"group_id":null}],
	  "unprotect_access_levels":[{"id":1,"access_level":60,"user_id":null,"group_id":null}],
	  "code_owner_approval_required":false,"inherited":false}]`

	branches := fetchBranches(t, eeJSON)

	level := branches[0].UnprotectAccessLevel
	if level == nil || *level != "admin" {
		t.Errorf("unprotect level = %v, want admin", level)
	}
}

// Recreating the protection from a document that dropped the grant would revoke it.
func TestFetchProtectedBranchesRejectsAnUnprotectGrantNoManifestCanHold(t *testing.T) {
	const grantedJSON = `[{"id":1,"name":"main",
	  "push_access_levels":[{"id":1,"access_level":40}],
	  "merge_access_levels":[{"id":1,"access_level":40}],
	  "unprotect_access_levels":[{"id":1,"access_level":40,"group_id":3}]}]`

	_, err := NewClient(protectedBranchRunner(grantedJSON, nil)).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)

	if !errors.Is(err, errIndescribableBranch) {
		t.Fatalf("err = %v, want an indescribable branch", err)
	}
	if !strings.Contains(err.Error(), "unprotect_access_levels") {
		t.Errorf("error = %q, want it to name the list it read", err)
	}
}

// An Enterprise build whose project holds no protection yet would otherwise be taken for a Community one.
func TestTheReadSaysWhetherTheInstanceReportsUnprotectLevels(t *testing.T) {
	for _, testCase := range []struct {
		name, listJSON string
		enterprise     bool
	}{
		{
			name: "community edition omits the field",
			listJSON: `[{"id":1,"name":"main",
			  "push_access_levels":[{"id":1,"access_level":40}],
			  "merge_access_levels":[{"id":1,"access_level":40}]}]`,
			enterprise: false,
		},
		{
			name:       "enterprise protecting nothing yet",
			listJSON:   `[]`,
			enterprise: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			read, err := NewClient(protectedBranchGlab{list: testCase.listJSON, enterprise: testCase.enterprise}).
				FetchProtectedBranches(context.Background(), ownerGroup, nameProj)
			if err != nil {
				t.Fatalf("fetch protected branches: %v", err)
			}
			if read.ReportsUnprotectLevels != testCase.enterprise {
				t.Errorf("reports unprotect levels = %t, want %t", read.ReportsUnprotectLevels, testCase.enterprise)
			}
		})
	}
}

// The edition decides what a declared unprotect level means, so it is not guessed.
func TestFetchProtectedBranchesFailsWhenTheVersionCannotBeRead(t *testing.T) {
	versionErr := errors.New("version unavailable")
	_, err := NewClient(protectedBranchGlab{list: protectedBranchListJSON, versionErr: versionErr}).
		FetchProtectedBranches(context.Background(), ownerGroup, nameProj)
	if !errors.Is(err, versionErr) {
		t.Errorf("err = %v, want the version failure", err)
	}
}
