package repository

import (
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// A nil slice is a document carrying no protected_branches key at all.
func declaringBranches(branches []manifest.ProtectedBranch) *manifest.Repository {
	return &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: ownerGroup, Name: nameProj},
		Spec:     manifest.RepositorySpec{ProtectedBranches: branches},
	}
}

// Read from an instance that reports no unprotect access level, which is what a Community Edition build answers.
// A nil slice is a project whose protections were not read at all.
func holdingBranches(branches []manifest.ProtectedBranch) *CurrentState {
	if branches == nil {
		return &CurrentState{Owner: ownerGroup, Name: nameProj}
	}
	return &CurrentState{
		Owner: ownerGroup, Name: nameProj,
		ProtectedBranches: &ProtectedBranches{Branches: branches},
	}
}

func holdingBranchesOnEnterprise(branches []manifest.ProtectedBranch) *CurrentState {
	return &CurrentState{
		Owner: ownerGroup, Name: nameProj,
		ProtectedBranches: &ProtectedBranches{Branches: branches, ReportsUnprotectLevels: true},
	}
}

func branch(name string, push manifest.BranchAccessLevel) manifest.ProtectedBranch {
	return manifest.ProtectedBranch{
		Name:             name,
		PushAccessLevel:  push,
		MergeAccessLevel: manifest.BranchMaintainer,
	}
}

func onlyBranchChange(t *testing.T, changes []Change) Change {
	t.Helper()
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly 1", changes)
	}
	if changes[0].ProtectedBranch == nil {
		t.Fatalf("change = %+v, want it to carry a protected branch", changes[0])
	}
	return changes[0]
}

// Without this a project would be stripped of its protections by any document that simply does not mention them.
func TestDiffLeavesLiveBranchesAloneWhenTheDocumentDeclaresNoKey(t *testing.T) {
	changes := Diff(
		declaringBranches(nil),
		holdingBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
	)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none for an undeclared key", changes)
	}
}

// Otherwise the plan would propose to create every branch the document names.
func TestDiffPlansNothingWhenTheLiveBranchesWereNotRead(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
		holdingBranches(nil),
	)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none when the live state is unknown", changes)
	}
}

func TestDiffCreatesADeclaredBranchTheProjectDoesNotProtect(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchNoOne)}),
		holdingBranches([]manifest.ProtectedBranch{}),
	)

	change := onlyBranchChange(t, changes)
	if change.Type != ChangeCreate {
		t.Errorf("type = %q, want a create", change.Type)
	}
	for _, want := range []string{
		`+ protected_branch "main"`, "+ push_access_level: no_one",
		"+ merge_access_level: maintainer", "+ allow_force_push: false",
	} {
		if !strings.Contains(change.String(), want) {
			t.Errorf("line = %q, want it to carry %q", change.String(), want)
		}
	}
}

func TestDiffRemovesALiveBranchNoDeclarationNames(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
	)

	change := onlyBranchChange(t, changes)
	if change.Type != ChangeDelete {
		t.Errorf("type = %q, want a delete", change.Type)
	}
	if change.String() != `- protected_branch "main"` {
		t.Errorf("line = %q, want the header alone", change.String())
	}
}

// The plan names only the attributes that changed, so the reader is not made to diff two full listings.
func TestDiffUpdatesABranchWhoseRoleChanged(t *testing.T) {
	want := branch("main", manifest.BranchNoOne)
	want.AllowForcePush = true

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
	)

	change := onlyBranchChange(t, changes)
	if change.Type != ChangeUpdate {
		t.Errorf("type = %q, want an update", change.Type)
	}
	line := change.String()
	for _, named := range []string{"push_access_level", "allow_force_push"} {
		if !strings.Contains(line, named) {
			t.Errorf("line = %q, want it to name %q", line, named)
		}
	}
	if strings.Contains(line, "merge_access_level") {
		t.Errorf("line = %q, want it to leave the unchanged attribute out", line)
	}
}

// An update recreates the protection, and GitLab creates one without code owner approval unless the create asks for it.
func TestDiffCarriesCodeOwnerApprovalIntoAnUpdate(t *testing.T) {
	live := holdingBranchesOnEnterprise([]manifest.ProtectedBranch{
		branch("main", manifest.BranchMaintainer), branch("release/*", manifest.BranchMaintainer),
	})
	live.ProtectedBranches.RequiringCodeOwnerApproval = []string{"main"}

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{
			branch("main", manifest.BranchNoOne),
			branch("release/*", manifest.BranchNoOne),
		}),
		live,
	)

	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want an update of each branch", changes)
	}
	for _, change := range changes {
		want := change.ProtectedBranch.Name == "main"
		if got := change.ProtectedBranch.CodeOwnerApprovalRequired; got != want {
			t.Errorf("%s: code owner approval carried = %t, want %t", change.ProtectedBranch.Name, got, want)
		}
	}
}

func TestDiffPlansNothingForAMatchingBranch(t *testing.T) {
	declared := []manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}

	changes := Diff(declaringBranches(declared), holdingBranches(declared))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// An update is applied by unprotecting and protecting again.
// A run that created first would meet a name GitLab still holds and be refused with 409.
func TestDiffPlansBranchRemovalsBeforeCreations(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("develop", manifest.BranchMaintainer)}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
	)

	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want a removal and a creation", changes)
	}
	if changes[0].Type != ChangeDelete || changes[1].Type != ChangeCreate {
		t.Errorf("order = %q then %q, want the removal first", changes[0].Type, changes[1].Type)
	}
}

// A wildcard is a name like any other, not a pattern to expand against the live branches.
func TestDiffPairsAWildcardBranchByItsName(t *testing.T) {
	declared := []manifest.ProtectedBranch{branch("release/*", manifest.BranchMaintainer)}

	changes := Diff(declaringBranches(declared), holdingBranches(declared))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// Without this a document exported from a CE build would report perpetual drift once pointed at an Enterprise one.
func TestDiffLeavesALiveUnprotectLevelAloneWhenTheDocumentIsSilent(t *testing.T) {
	live := branch("main", manifest.BranchMaintainer)
	live.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("admin"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
		holdingBranchesOnEnterprise([]manifest.ProtectedBranch{live}),
	)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

func TestDiffUpdatesADeclaredUnprotectLevel(t *testing.T) {
	want := branch("main", manifest.BranchMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("admin"))
	live := branch("main", manifest.BranchMaintainer)
	live.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("maintainer"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranchesOnEnterprise([]manifest.ProtectedBranch{live}),
	)

	change := onlyBranchChange(t, changes)
	if !strings.Contains(change.String(), "unprotect_access_level: maintainer") {
		t.Errorf("line = %q, want it to name the unprotect level", change.String())
	}
}

// An unprotect level nobody wrote takes whatever GitLab grants, which the plan cannot name.
func TestDiffOmitsAnUnstatedUnprotectLevelFromACreation(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
		holdingBranches([]manifest.ProtectedBranch{}),
	)

	change := onlyBranchChange(t, changes)
	if strings.Contains(change.String(), "unprotect_access_level") {
		t.Errorf("line = %q, want it to leave the unstated level out", change.String())
	}
}

// The apply would recreate the branch and the next read would report nothing again.
// The plan would say the same thing forever while reopening the branch each time.
func TestDiffPlansNothingForAnUnprotectLevelTheInstanceCannotReport(t *testing.T) {
	want := branch("main", manifest.BranchMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("maintainer"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
	)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none against an instance that cannot confirm the level", changes)
	}
}

// An Enterprise build reports an empty unprotect list for a protection created with its project.
// A declared level there is drift the apply can settle, unlike on a CE build.
func TestDiffUpdatesAnUnprotectLevelTheInstanceReportsAsUnset(t *testing.T) {
	want := branch("main", manifest.BranchMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("admin"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranchesOnEnterprise([]manifest.ProtectedBranch{branch("main", manifest.BranchMaintainer)}),
	)

	change := onlyBranchChange(t, changes)
	if !strings.Contains(change.String(), "unprotect_access_level: (none) → admin") {
		t.Errorf("line = %q, want it to read as a change from nothing", change.String())
	}
}

func TestUnconfirmableUnprotectLevelsNamesTheBranchesItIgnores(t *testing.T) {
	want := branch("main", manifest.BranchMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("maintainer"))
	declared := declaringBranches([]manifest.ProtectedBranch{want, branch("develop", manifest.BranchMaintainer)})
	live := []manifest.ProtectedBranch{
		branch("main", manifest.BranchMaintainer),
		branch("develop", manifest.BranchMaintainer),
	}

	if got := UnconfirmableUnprotectLevels(declared, holdingBranches(live)); !slices.Equal(got, []string{"main"}) {
		t.Errorf("ignored = %v, want the branch declaring a level the instance cannot report", got)
	}
	if got := UnconfirmableUnprotectLevels(declared, holdingBranchesOnEnterprise(live)); got != nil {
		t.Errorf("ignored = %v, want none where the instance reports the field", got)
	}
}
