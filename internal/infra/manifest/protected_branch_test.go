package manifest

import (
	"reflect"
	"strings"
	"testing"
)

// One fully spelled-out protected branch.
// A decode test can assert every attribute of it without a default standing in for a declared one.
const protectedBranchDoc = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  topics: []
  protected_branches:
    - name: main
      push_access_level: no_one
      merge_access_level: developer
      allow_force_push: true
`

// Every attribute a document spells out survives decoding.
// A declared protection is read as written rather than as the defaults below.
func TestParseReadsProtectedBranches(t *testing.T) {
	repos, errs := Parse([]byte(protectedBranchDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	want := []ProtectedBranch{{
		Name:             "main",
		PushAccessLevel:  "no_one",
		MergeAccessLevel: "developer",
		AllowForcePush:   true,
	}}
	if got := repos[0].Spec.ProtectedBranches; !reflect.DeepEqual(got, want) {
		t.Errorf("branches = %+v, want %+v", got, want)
	}
}

// A branch declaring only its name takes what GitLab would grant on create.
// The manifest and the live state then agree about a protection nobody spelled out.
func TestParseFillsOmittedProtectedBranchAttributesWithGitLabDefaults(t *testing.T) {
	doc := validDoc + `  protected_branches:
    - name: main
`
	repos, errs := Parse([]byte(doc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	branch := repos[0].Spec.ProtectedBranches[0]
	if branch.PushAccessLevel != BranchMaintainer {
		t.Errorf("push_access_level = %q, want %q", branch.PushAccessLevel, BranchMaintainer)
	}
	if branch.MergeAccessLevel != BranchMaintainer {
		t.Errorf("merge_access_level = %q, want %q", branch.MergeAccessLevel, BranchMaintainer)
	}
	if branch.AllowForcePush {
		t.Error("allow_force_push = true, want false")
	}
}

// The wildcard patterns GitLab accepts are branch names as far as the manifest is concerned.
// Nothing rewrites them the way a bare ref is expanded.
func TestParseKeepsAProtectedBranchWildcardVerbatim(t *testing.T) {
	doc := validDoc + `  protected_branches:
    - name: release/*
`
	repos, errs := Parse([]byte(doc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if name := repos[0].Spec.ProtectedBranches[0].Name; name != "release/*" {
		t.Errorf("name = %q, want it kept verbatim", name)
	}
}

// The two ways of writing "no protected branches" mean different things, so decoding must keep them apart.
// An omitted block leaves the live protections alone, while an empty list declares that none should exist.
func TestParseDistinguishesOmittedProtectedBranchesFromAnEmptyList(t *testing.T) {
	omitted, errs := Parse([]byte(validDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if branches := omitted[0].Spec.ProtectedBranches; branches != nil {
		t.Errorf("omitted block = %v, want nil (unmanaged)", branches)
	}

	declared, errs := Parse([]byte(validDoc + "  protected_branches: []\n"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	branches := declared[0].Spec.ProtectedBranches
	if branches == nil {
		t.Fatal("empty list = nil, want an empty slice (declares none)")
	}
	if len(branches) != 0 {
		t.Errorf("empty list = %v, want no elements", branches)
	}
}

// A branch decodes itself.
// The strict decoding the surrounding document gets does not reach inside it unless the nested decode asks for it too.
// The key most likely to be tried is the one GitLab reports and this schema does not.
func TestParseRejectsUnknownKeyInsideAProtectedBranch(t *testing.T) {
	doc := validDoc + `  protected_branches:
    - name: main
      code_owner_approval_required: true
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
}

// A document exported from CE then says nothing about it instead of demanding a value GitLab would never confirm.
func TestParseDistinguishesAnOmittedUnprotectLevelFromADeclaredOne(t *testing.T) {
	omitted, errs := Parse([]byte(validDoc + "  protected_branches:\n    - name: main\n"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if level := omitted[0].Spec.ProtectedBranches[0].UnprotectAccessLevel; level != nil {
		t.Errorf("omitted key = %v, want nil (unmanaged)", *level)
	}

	declared, errs := Parse([]byte(validDoc +
		"  protected_branches:\n    - name: main\n      unprotect_access_level: admin\n"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	level := declared[0].Spec.ProtectedBranches[0].UnprotectAccessLevel
	if level == nil || *level != BranchUnprotectAccessLevel(BranchAdmin) {
		t.Errorf("declared key = %v, want admin", level)
	}
}

// GitLab answers an unprotect access level of 0 with 400.
func TestParseRejectsAnUnprotectLevelOfNoOne(t *testing.T) {
	doc := validDoc + `  protected_branches:
    - name: main
      unprotect_access_level: no_one
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "developer, maintainer, admin") {
		t.Errorf("error = %q, want it to list the allowed values", errs[0])
	}
}

// The name is what GitLab addresses a protection by.
// One left empty could neither be created nor paired with a live branch.
// The error locates it by position, because nothing else identifies it.
func TestParseRequiresAProtectedBranchName(t *testing.T) {
	doc := validDoc + `  protected_branches:
    - name: main
    - push_access_level: developer
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "spec.protected_branches[1].name") {
		t.Errorf("error = %q, want it to locate the unnamed branch", errs[0])
	}
}

// GitLab answers a second protection of the same name with 409, so a document declaring one twice could never apply.
// The error names the branch because that is what the reader has to go and change.
func TestParseRejectsDuplicateProtectedBranchNames(t *testing.T) {
	doc := validDoc + `  protected_branches:
    - name: main
    - name: develop
    - name: main
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "main") {
		t.Errorf("error = %q, want it to name the repeated branch", errs[0])
	}
}

// An empty protected-branch list survives Marshal as "protected_branches: []".
// It is not dropped the way an unset value is, so an export of a project holding none says so.
func TestMarshalKeepsAnEmptyProtectedBranchList(t *testing.T) {
	doc := fullRepository()
	doc.Spec.ProtectedBranches = []ProtectedBranch{}

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "protected_branches: []") {
		t.Errorf("emitted document does not declare an empty branch list:\n%s", out)
	}
}

// A document that says nothing about the protections emits no key, and parsing that output says nothing again.
func TestMarshalOmitsUndeclaredProtectedBranches(t *testing.T) {
	doc := fullRepository()
	doc.Spec.ProtectedBranches = nil

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "protected_branches") {
		t.Errorf("emitted document names the branches it says nothing about:\n%s", out)
	}
}

// A branch whose push is denied to everyone is the reason "no_one" exists.
// It is also the value a boolean-shaped default would quietly turn into something else.
// It therefore has to survive the whole round trip.
func TestMarshalKeepsABranchNobodyMayPushTo(t *testing.T) {
	doc := fullRepository()
	doc.Spec.ProtectedBranches = []ProtectedBranch{{
		Name:             "main",
		PushAccessLevel:  BranchNoOne,
		MergeAccessLevel: BranchMaintainer,
	}}

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	repos, errs := Parse(out)
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if got := repos[0].Spec.ProtectedBranches[0].PushAccessLevel; got != BranchNoOne {
		t.Errorf("push_access_level = %q, want %q", got, BranchNoOne)
	}
}
