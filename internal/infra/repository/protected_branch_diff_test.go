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

// Live state holding the given protections, read from an instance that reports
// no unprotect access level, which is what a Community Edition build answers. A
// nil slice is a project whose protections were not read at all.
func holdingBranches(branches []manifest.ProtectedBranch) *CurrentState {
	if branches == nil {
		return &CurrentState{Owner: ownerGroup, Name: nameProj}
	}
	return &CurrentState{
		Owner: ownerGroup, Name: nameProj,
		ProtectedBranches: &ProtectedBranches{Branches: branches},
	}
}

// The same, read from an instance that does report them.
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
		MergeAccessLevel: roleMaintainer,
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
	changes := Diff(declaringBranches(nil), holdingBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none for an undeclared key", changes)
	}
}

// Otherwise the plan would propose to create every branch the document names.
func TestDiffPlansNothingWhenTheLiveBranchesWereNotRead(t *testing.T) {
	changes := Diff(declaringBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}), holdingBranches(nil))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none when the live state is unknown", changes)
	}
}

func TestDiffCreatesADeclaredBranchTheProjectDoesNotProtect(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", roleNoOne)}),
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
		holdingBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
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
	want := branch("main", roleNoOne)
	want.AllowForcePush = true

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
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

func TestDiffPlansNothingForAMatchingBranch(t *testing.T) {
	declared := []manifest.ProtectedBranch{branch("main", roleMaintainer)}

	changes := Diff(declaringBranches(declared), holdingBranches(declared))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// An update is applied by unprotecting and protecting again.
// A run that created first would meet a name GitLab still holds and be refused with 409.
func TestDiffPlansBranchRemovalsBeforeCreations(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("develop", roleMaintainer)}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
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
	declared := []manifest.ProtectedBranch{branch("release/*", roleMaintainer)}

	changes := Diff(declaringBranches(declared), holdingBranches(declared))

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// A declaration silent about the unprotect level is not a demand that it be
// cleared, so a live one is left alone rather than planned as drift. Without
// this a document exported from a CE build would report perpetual drift the
// moment it was pointed at an Enterprise one.
func TestDiffLeavesALiveUnprotectLevelAloneWhenTheDocumentIsSilent(t *testing.T) {
	live := branch("main", roleMaintainer)
	live.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("admin"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
		holdingBranchesOnEnterprise([]manifest.ProtectedBranch{live}),
	)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// A declared unprotect level that differs is drift like any other attribute,
// and the plan names it.
func TestDiffUpdatesADeclaredUnprotectLevel(t *testing.T) {
	want := branch("main", roleMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("admin"))
	live := branch("main", roleMaintainer)
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

// A creation lists only what the declaration states. An unprotect level nobody
// wrote takes whatever GitLab grants, which the plan cannot name, so showing it
// empty would claim knowledge the tool does not have.
func TestDiffOmitsAnUnstatedUnprotectLevelFromACreation(t *testing.T) {
	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
		holdingBranches([]manifest.ProtectedBranch{}),
	)

	change := onlyBranchChange(t, changes)
	if strings.Contains(change.String(), "unprotect_access_level") {
		t.Errorf("line = %q, want it to leave the unstated level out", change.String())
	}
}

// A CE build never reports an unprotect level, so a manifest declaring one
// against such an instance must not be planned as drift: the apply would
// recreate the branch, the next read would report nothing again, and the plan
// would say the same thing forever while reopening the branch each time.
func TestDiffPlansNothingForAnUnprotectLevelTheInstanceCannotReport(t *testing.T) {
	want := branch("main", roleMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("maintainer"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranches([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
	)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none against an instance that cannot confirm the level", changes)
	}
}

// An Enterprise build reports an empty unprotect list for a protection created
// with its project, so a declared level there is drift the apply can settle: the
// recreate sends it and the next read reports it back.
func TestDiffUpdatesAnUnprotectLevelTheInstanceReportsAsUnset(t *testing.T) {
	want := branch("main", roleMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("admin"))

	changes := Diff(
		declaringBranches([]manifest.ProtectedBranch{want}),
		holdingBranchesOnEnterprise([]manifest.ProtectedBranch{branch("main", roleMaintainer)}),
	)

	change := onlyBranchChange(t, changes)
	if !strings.Contains(change.String(), "unprotect_access_level: (none) → admin") {
		t.Errorf("line = %q, want it to read as a change from nothing", change.String())
	}
}

// A declaration the instance cannot confirm is named, so a plan can say the key
// is being left alone rather than show nothing for a key the operator wrote.
func TestUnconfirmableUnprotectLevelsNamesTheBranchesItIgnores(t *testing.T) {
	want := branch("main", roleMaintainer)
	want.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel("maintainer"))
	declared := declaringBranches([]manifest.ProtectedBranch{want, branch("develop", roleMaintainer)})
	live := []manifest.ProtectedBranch{branch("main", roleMaintainer), branch("develop", roleMaintainer)}

	if got := UnconfirmableUnprotectLevels(declared, holdingBranches(live)); !slices.Equal(got, []string{"main"}) {
		t.Errorf("ignored = %v, want the branch declaring a level the instance cannot report", got)
	}
	if got := UnconfirmableUnprotectLevels(declared, holdingBranchesOnEnterprise(live)); got != nil {
		t.Errorf("ignored = %v, want none where the instance reports the field", got)
	}
}
