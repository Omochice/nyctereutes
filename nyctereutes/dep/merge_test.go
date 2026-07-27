package dep_test

import (
	"strings"
	"testing"
)

func TestDepMergeRequiresGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runWithRunner(fake, "dep", "merge")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stderr == "" {
		t.Error("want an error on stderr")
	}
}

func TestDepMergeInvalidMethod(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runWithRunner(fake, "dep", "merge", "--group", "lodash@2.0.0", "--method", "bogus")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stderr == "" {
		t.Error("want an error on stderr")
	}
	if len(fake.merged) != 0 {
		t.Errorf("must not merge on invalid method, got %d calls", len(fake.merged))
	}
}

func TestDepMergeAutoMergeByDefault(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "merge", "--group", "lodash@2.0.0")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(fake.merged) != 1 {
		t.Fatalf("merge called %d times, want 1", len(fake.merged))
	}
	args := strings.Join(fake.merged[0], " ")
	if !strings.Contains(args, "--squash") || !strings.Contains(args, "--auto-merge") {
		t.Errorf("default merge args = %q, want --squash and --auto-merge", args)
	}
	if !strings.Contains(stdout, "auto-merge when pipeline succeeds") {
		t.Errorf("want auto-merge message, got %q", stdout)
	}
}

func TestDepMergeImmediate(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "merge", "--group", "lodash@2.0.0", "--require-checks=false")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(fake.merged) != 1 {
		t.Fatalf("merge called %d times, want 1", len(fake.merged))
	}
	args := strings.Join(fake.merged[0], " ")
	if !strings.Contains(args, "--auto-merge=false") {
		t.Errorf("immediate merge args = %q, want --auto-merge=false", args)
	}
	if strings.Contains(stdout, "auto-merge when pipeline succeeds") {
		t.Errorf("immediate merge should not print auto-merge message, got %q", stdout)
	}
}

func TestDepMergeContinuesOnError(t *testing.T) {
	fake := &fakeGlab{listJSON: twoMRsSameGroup, detailJSON: `{}`, mergeErr: errStub409}
	exit, stdout, _ := runWithRunner(fake, "dep", "merge", "--group", "lodash@2.0.0")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1 (failures must be non-zero exit)", exit)
	}
	if len(fake.merged) != 2 {
		t.Errorf("want both MRs attempted, got %d", len(fake.merged))
	}
	if !strings.Contains(stdout, "failed to merge") {
		t.Errorf("want failure reported, got %q", stdout)
	}
}

func TestDepMergeDryRun(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "merge", "--group", "lodash@2.0.0", "--dry-run")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(fake.merged) != 0 {
		t.Errorf("dry-run must not merge, got %d calls", len(fake.merged))
	}
	if !strings.Contains(stdout, "merge !12") {
		t.Errorf("want planned merge printed, got %q", stdout)
	}
}
