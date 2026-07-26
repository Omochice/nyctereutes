package dep_test

import (
	"strings"
	"testing"
)

func TestDepApproveRequiresGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runWithRunner(fake, "dep", "approve")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stderr == "" {
		t.Error("want an error on stderr")
	}
}

func TestDepApproveApprovesGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "approve", "--group", "lodash@2.0.0")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(fake.approved) != 1 {
		t.Fatalf("approve called %d times, want 1", len(fake.approved))
	}
	if !strings.Contains(stdout, "approve !12") {
		t.Errorf("want approve action, got %q", stdout)
	}
}

func TestDepApproveDryRun(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "approve", "--group", "lodash@2.0.0", "--dry-run")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(fake.approved) != 0 {
		t.Errorf("dry-run must not approve, got %d calls", len(fake.approved))
	}
	if !strings.Contains(stdout, "approve !12") {
		t.Errorf("want planned action printed, got %q", stdout)
	}
}

func TestDepApproveGroupNotFound(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runWithRunner(fake, "dep", "approve", "--group", "missing@9.9.9")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stderr == "" {
		t.Error("want an error on stderr")
	}
	if len(fake.approved) != 0 {
		t.Errorf("must not approve when group missing, got %d calls", len(fake.approved))
	}
}

func TestDepApproveContinuesOnError(t *testing.T) {
	fake := &fakeGlab{listJSON: twoMRsSameGroup, detailJSON: `{}`, approveErr: errStub500}
	exit, stdout, _ := runWithRunner(fake, "dep", "approve", "--group", "lodash@2.0.0")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1 (failures must be non-zero exit)", exit)
	}
	if len(fake.approved) != 2 {
		t.Errorf("want both MRs attempted, got %d", len(fake.approved))
	}
	if !strings.Contains(stdout, "failed to approve") {
		t.Errorf("want failure reported, got %q", stdout)
	}
}
