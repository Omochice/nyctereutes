package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The change one protected-branch write acts on, in the form the diff hands to
// the applier.
func branchChange(kind ChangeType, change ProtectedBranchChange) Change {
	return protectedBranchChange(kind, "group/proj", change)
}

// Applies one change and reports the calls it made, failing the test when the
// write itself failed.
func applyBranch(t *testing.T, writer *recordingWriter, change Change) []writerCall {
	t.Helper()
	results := NewApplier(writer).Apply(context.Background(), []Change{change})
	if len(results) != 1 {
		t.Fatalf("results = %+v, want one", results)
	}
	if results[0].Err != nil {
		t.Fatalf("apply: %v", results[0].Err)
	}
	return writer.calls
}

// A creation posts to the collection with every attribute, the roles carried as
// the numbers GitLab wants rather than the names the manifest holds.
func TestApplyProtectsADeclaredBranch(t *testing.T) {
	writer := &recordingWriter{}
	change := branchChange(ChangeCreate, ProtectedBranchChange{
		Name: "main",
		Desired: manifest.ProtectedBranch{
			Name:             "main",
			PushAccessLevel:  roleNoOne,
			MergeAccessLevel: roleDeveloper,
			AllowForcePush:   true,
		},
	})

	calls := applyBranch(t, writer, change)

	if len(calls) != 1 {
		t.Fatalf("calls = %+v, want one", calls)
	}
	want := "api projects/group%2Fproj/protected_branches --method POST " +
		"-f name=main -f push_access_level=0 -f merge_access_level=30 -f allow_force_push=true"
	if got := strings.Join(calls[0].args, " "); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// An update unprotects and protects again. GitLab's PATCH answers 200 to a
// changed push_access_level and changes nothing, so a write built on it would
// report a change it did not make.
func TestApplyReprotectsABranchWhoseRoleChanged(t *testing.T) {
	writer := &recordingWriter{}
	change := branchChange(ChangeUpdate, ProtectedBranchChange{
		Name: "main",
		Desired: manifest.ProtectedBranch{
			Name: "main", PushAccessLevel: roleNoOne, MergeAccessLevel: roleMaintainer,
		},
		Live: manifest.ProtectedBranch{
			Name: "main", PushAccessLevel: roleMaintainer, MergeAccessLevel: roleMaintainer,
		},
	})

	calls := applyBranch(t, writer, change)

	if len(calls) != 2 {
		t.Fatalf("calls = %+v, want an unprotect and a protect", calls)
	}
	wantFirst := "api projects/group%2Fproj/protected_branches/main --method DELETE"
	if got := strings.Join(calls[0].args, " "); got != wantFirst {
		t.Errorf("first call = %q, want the unprotect %q", got, wantFirst)
	}
	if !strings.Contains(strings.Join(calls[1].args, " "), "--method POST") {
		t.Errorf("second call = %q, want the protect", calls[1].args)
	}
}

// A removal unprotects the branch and does nothing else.
func TestApplyUnprotectsABranchNoDeclarationNames(t *testing.T) {
	writer := &recordingWriter{}

	calls := applyBranch(t, writer, branchChange(ChangeDelete, ProtectedBranchChange{Name: "main"}))

	if len(calls) != 1 {
		t.Fatalf("calls = %+v, want one", calls)
	}
	want := "api projects/group%2Fproj/protected_branches/main --method DELETE"
	if got := strings.Join(calls[0].args, " "); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// A protected branch is routinely named with a slash, and GitLab answers an
// unencoded path 404. Without the encoding an update would delete nothing and
// then be refused for a name GitLab still holds.
func TestApplyEncodesASlashInABranchName(t *testing.T) {
	writer := &recordingWriter{}

	calls := applyBranch(t, writer, branchChange(ChangeDelete, ProtectedBranchChange{Name: "release/1.x"}))

	want := "api projects/group%2Fproj/protected_branches/release%2F1.x --method DELETE"
	if got := strings.Join(calls[0].args, " "); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// A re-protect that fails after its unprotect succeeded leaves the branch open,
// which nothing else in the report would say. The error names that outcome so
// the operator knows to go and look rather than only that a write failed.
func TestApplyReportsABranchLeftUnprotectedByAFailedReprotect(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{1: errBoom}}
	change := branchChange(ChangeUpdate, ProtectedBranchChange{
		Name: "main",
		Desired: manifest.ProtectedBranch{
			Name: "main", PushAccessLevel: roleNoOne, MergeAccessLevel: roleMaintainer,
		},
	})

	results := NewApplier(writer).Apply(context.Background(), []Change{change})

	if !errors.Is(results[0].Err, errBranchLeftUnprotected) {
		t.Fatalf("err = %v, want it to report the branch left unprotected", results[0].Err)
	}
	if !strings.Contains(results[0].Err.Error(), "main") {
		t.Errorf("error = %q, want it to name the branch", results[0].Err)
	}
}

// An unprotect that fails is reported as an ordinary failed write: the branch
// still holds the protection it had, so nothing was lost.
func TestApplyReportsAFailedUnprotectWithoutClaimingTheBranchIsOpen(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}

	results := NewApplier(writer).Apply(context.Background(),
		[]Change{branchChange(ChangeDelete, ProtectedBranchChange{Name: "main"})})

	if results[0].Err == nil {
		t.Fatal("err = nil, want the failed write reported")
	}
	if errors.Is(results[0].Err, errBranchLeftUnprotected) {
		t.Errorf("error = %q, want it not to claim the protection was lost", results[0].Err)
	}
}
