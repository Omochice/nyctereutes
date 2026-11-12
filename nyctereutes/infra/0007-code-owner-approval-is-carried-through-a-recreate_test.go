package infra_test

import (
	"strconv"
	"strings"
	"testing"
)

func protectionReportingCodeOwnerApproval(required bool) string {
	return `[{"id":1,"name":"main",
	  "push_access_levels":[{"id":1,"access_level":40,"user_id":null,"group_id":null}],
	  "merge_access_levels":[{"id":1,"access_level":40,"user_id":null,"group_id":null}],
	  "unprotect_access_levels":[{"id":1,"access_level":40,"user_id":null,"group_id":null}],
	  "allow_force_push":false,
	  "code_owner_approval_required":` + strconv.FormatBool(required) + `,"inherited":false}]`
}

func writesOfARoleChange(t *testing.T, codeOwnerApprovalRequired bool) []string {
	t.Helper()
	path := writeManifest(t, t.TempDir(), "a.yaml", planBranchManifest)
	runner := &fakeApplyGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		branches: map[string]string{
			targetGroupProj: protectionReportingCodeOwnerApproval(codeOwnerApprovalRequired),
		},
		enterprise: true,
	}

	exit, _, stderr := runInfraApply("y\n", runner, "infra", "apply", path)

	if exit != 0 {
		t.Fatalf("exit = %d (stderr %q), want 0", exit, stderr)
	}
	if len(runner.writes) != 2 {
		t.Fatalf("writes = %q, want an unprotect and a protect", runner.writes)
	}
	return runner.writes
}

func TestARecreateSendsCodeOwnerApprovalAgainWhenTheInstanceReportedItOn(t *testing.T) {
	protect := writesOfARoleChange(t, true)[1]

	if !strings.Contains(protect, "--method POST") {
		t.Fatalf("second write = %q, want the protect", protect)
	}
	if !strings.Contains(protect, "code_owner_approval_required=true") {
		t.Errorf("protect = %q, want it to ask for code owner approval again", protect)
	}
}

func TestARecreateSendsNoCodeOwnerApprovalWhenTheInstanceReportedItOff(t *testing.T) {
	for _, write := range writesOfARoleChange(t, false) {
		if strings.Contains(write, "code_owner_approval_required") {
			t.Errorf("write = %q, want the setting left off the request", write)
		}
	}
}

func TestAnExportWritesNoCodeOwnerApproval(t *testing.T) {
	runner := &fakeInfraGlab{
		projects:   map[string]string{targetGroupProj: projJSON},
		schedules:  map[string]string{},
		branches:   map[string]string{targetGroupProj: protectionReportingCodeOwnerApproval(true)},
		enterprise: true,
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", exit, stderr)
	}
	if !strings.Contains(stdout, "protected_branches:") {
		t.Fatalf("stdout carries no protected branch to look at\n%s", stdout)
	}
	if strings.Contains(stdout, "code_owner_approval") {
		t.Errorf("the export names a setting no manifest key holds\n%s", stdout)
	}
}
