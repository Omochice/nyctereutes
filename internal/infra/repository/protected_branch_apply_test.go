package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

func branchChange(kind ChangeType, change ProtectedBranchChange) Change {
	return protectedBranchChange(kind, "group/proj", change)
}

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

// GitLab's PATCH answers 200 to a changed push_access_level and changes nothing.
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

// Without the encoding an update would delete nothing and then be refused for a name GitLab still holds.
func TestApplyEncodesASlashInABranchName(t *testing.T) {
	writer := &recordingWriter{}

	calls := applyBranch(t, writer, branchChange(ChangeDelete, ProtectedBranchChange{Name: "release/1.x"}))

	want := "api projects/group%2Fproj/protected_branches/release%2F1.x --method DELETE"
	if got := strings.Join(calls[0].args, " "); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

// GitLab CE 19.5.3 unprotects a wildcard addressed with its asterisk percent-encoded, and protects one sent verbatim.
func TestApplyReprotectsAWildcardByItsName(t *testing.T) {
	writer := &recordingWriter{}
	change := branchChange(ChangeUpdate, ProtectedBranchChange{
		Name: "release/*",
		Desired: manifest.ProtectedBranch{
			Name: "release/*", PushAccessLevel: roleDeveloper, MergeAccessLevel: roleMaintainer,
		},
		Live: manifest.ProtectedBranch{
			Name: "release/*", PushAccessLevel: roleMaintainer, MergeAccessLevel: roleMaintainer,
		},
	})

	calls := applyBranch(t, writer, change)

	want := []string{
		"api projects/group%2Fproj/protected_branches/release%2F%2A --method DELETE",
		"api projects/group%2Fproj/protected_branches --method POST " +
			"-f name=release/* -f push_access_level=30 -f merge_access_level=40 -f allow_force_push=false",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %+v, want %d", calls, len(want))
	}
	for i, call := range calls {
		if got := strings.Join(call.args, " "); got != want[i] {
			t.Errorf("call %d = %q, want %q", i, got, want[i])
		}
	}
}

// GitLab CE 19.5.3 accepts every one of these as a protection's name, a run of asterisks included.
func TestApplyUnprotectsEveryWildcardShapeGitLabAccepts(t *testing.T) {
	for name, encoded := range map[string]string{
		"*":          "%2A",
		"**":         "%2A%2A",
		"***":        "%2A%2A%2A",
		"*-stable":   "%2A-stable",
		"release/**": "release%2F%2A%2A",
	} {
		writer := &recordingWriter{}

		calls := applyBranch(t, writer, branchChange(ChangeDelete, ProtectedBranchChange{Name: name}))

		want := "api projects/group%2Fproj/protected_branches/" + encoded + " --method DELETE"
		if got := strings.Join(calls[0].args, " "); got != want {
			t.Errorf("%s: glab args = %q, want %q", name, got, want)
		}
	}
}

// Nothing else in the report would say that the branch is open.
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

// The branch still holds the protection it had, so nothing was lost.
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

// A recreate carries the live unprotect level onto a declaration that is silent about it.
// Without this an update would send no unprotect level and GitLab would grant its own default.
// A branch only instance admins could unprotect would quietly fall to maintainers with nothing in the plan to say so.
func TestApplyCarriesAnUnstatedUnprotectLevelThroughARecreate(t *testing.T) {
	writer := &recordingWriter{}
	live := manifest.ProtectedBranch{
		Name: "main", PushAccessLevel: roleMaintainer, MergeAccessLevel: roleMaintainer,
		UnprotectAccessLevel: new(manifest.BranchUnprotectAccessLevel("admin")),
	}
	change := branchChange(ChangeUpdate, ProtectedBranchChange{
		Name: "main",
		Desired: manifest.ProtectedBranch{
			Name: "main", PushAccessLevel: roleNoOne, MergeAccessLevel: roleMaintainer,
		},
		Live: live,
	})

	calls := applyBranch(t, writer, change)

	if len(calls) != 2 {
		t.Fatalf("calls = %+v, want an unprotect and a protect", calls)
	}
	if !strings.Contains(strings.Join(calls[1].args, " "), "unprotect_access_level=60") {
		t.Errorf("protect = %q, want it to carry the live unprotect level", calls[1].args)
	}
}

// A declared unprotect level goes on the request, overriding the live one.
func TestApplySendsADeclaredUnprotectLevel(t *testing.T) {
	writer := &recordingWriter{}
	change := branchChange(ChangeCreate, ProtectedBranchChange{
		Name: "main",
		Desired: manifest.ProtectedBranch{
			Name: "main", PushAccessLevel: roleMaintainer, MergeAccessLevel: roleMaintainer,
			UnprotectAccessLevel: new(manifest.BranchUnprotectAccessLevel("developer")),
		},
	})

	calls := applyBranch(t, writer, change)

	if !strings.Contains(strings.Join(calls[0].args, " "), "unprotect_access_level=30") {
		t.Errorf("protect = %q, want it to carry the declared unprotect level", calls[0].args)
	}
}

// A CE build reports no unprotect level, so nothing states one and nothing is sent.
// A value GitLab would never report back could not be planned against.
func TestApplySendsNoUnprotectLevelWhenNobodyStatesOne(t *testing.T) {
	writer := &recordingWriter{}
	change := branchChange(ChangeCreate, ProtectedBranchChange{
		Name: "main",
		Desired: manifest.ProtectedBranch{
			Name: "main", PushAccessLevel: roleMaintainer, MergeAccessLevel: roleMaintainer,
		},
	})

	calls := applyBranch(t, writer, change)

	if strings.Contains(strings.Join(calls[0].args, " "), "unprotect_access_level") {
		t.Errorf("protect = %q, want no unprotect level sent", calls[0].args)
	}
}
