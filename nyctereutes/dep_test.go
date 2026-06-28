package nyctereutes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
)

var detailPath = regexp.MustCompile(`merge_requests/\d+$`)

var (
	errStub500 = errors.New("500 Internal Server Error")
	errStub409 = errors.New("409 Conflict")
)

// fakeGlab scripts glab responses and records destructive calls.
type fakeGlab struct {
	mu         sync.Mutex
	listJSON   string
	detailJSON string
	approveErr error
	mergeErr   error
	approved   [][]string
	merged     [][]string
}

func (fake *fakeGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()

	switch {
	case args[0] == "config":
		return nil, nil // unset -> defaults apply
	case args[0] == "api":
		path := args[len(args)-1]
		if detailPath.MatchString(path) {
			return []byte(fake.detailJSON), nil
		}
		return []byte(fake.listJSON), nil
	case args[0] == "mr" && args[1] == "approve":
		fake.approved = append(fake.approved, args)
		return nil, fake.approveErr
	case args[0] == "mr" && args[1] == "merge":
		fake.merged = append(fake.merged, args)
		return nil, fake.mergeErr
	}
	return nil, nil
}

func runDep(runner glab.Runner, args ...string) (exit int, stdout, stderr string) {
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	exit = dispatch(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	}, runner)
	return exit, outBuf.String(), errBuf.String()
}

const oneMR = `[{"iid":12,"project_id":7,"title":"Bump lodash from 1.0.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12"}]`

const twoMRsSameGroup = `[` +
	`{"iid":12,"project_id":7,"title":"Bump lodash from 1.0.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12"},` +
	`{"iid":13,"project_id":8,"title":"Bump lodash from 1.1.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/other/-/merge_requests/13"}]`

func TestDepListRendersTable(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runDep(fake, "dep", "list")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	for _, want := range []string{"PROJECT", "MR", "TITLE", "g/proj", "!12", "Bump lodash"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("list output missing %q\n%s", want, stdout)
		}
	}
}

func TestDepListGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runDep(fake, "dep", "list", "--group")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "GROUP") || !strings.Contains(stdout, "lodash@2.0.0") {
		t.Errorf("group output missing GROUP/key\n%s", stdout)
	}
}

func TestDepListJSON(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runDep(fake, "dep", "list", "--json")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("list --json is not valid JSON: %v\n%s", err, stdout)
	}
}

func TestDepListEmpty(t *testing.T) {
	fake := &fakeGlab{listJSON: `[]`, detailJSON: `{}`}
	exit, stdout, _ := runDep(fake, "dep", "list")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "No dependency MRs found") {
		t.Errorf("want empty message, got %q", stdout)
	}
}

func TestDepListEmptyJSON(t *testing.T) {
	fake := &fakeGlab{listJSON: `[]`, detailJSON: `{}`}
	exit, stdout, _ := runDep(fake, "dep", "list", "--json")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if strings.Contains(stdout, "No dependency MRs found") {
		t.Errorf("--json must not emit the plain message, got %q", stdout)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("empty --json is not valid JSON: %v\n%s", err, stdout)
	}
}

func TestDepApproveRequiresGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runDep(fake, "dep", "approve")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stderr == "" {
		t.Error("want an error on stderr")
	}
}

func TestDepApproveApprovesGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runDep(fake, "dep", "approve", "--group", "lodash@2.0.0")
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
	exit, stdout, _ := runDep(fake, "dep", "approve", "--group", "lodash@2.0.0", "--dry-run")
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
	exit, _, stderr := runDep(fake, "dep", "approve", "--group", "missing@9.9.9")
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
	exit, stdout, _ := runDep(fake, "dep", "approve", "--group", "lodash@2.0.0")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if len(fake.approved) != 2 {
		t.Errorf("want both MRs attempted, got %d", len(fake.approved))
	}
	if !strings.Contains(stdout, "failed to approve") {
		t.Errorf("want failure reported, got %q", stdout)
	}
}

func TestDepMergeRequiresGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runDep(fake, "dep", "merge")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stderr == "" {
		t.Error("want an error on stderr")
	}
}

func TestDepMergeInvalidMethod(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, _, stderr := runDep(fake, "dep", "merge", "--group", "lodash@2.0.0", "--method", "bogus")
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
	exit, stdout, _ := runDep(fake, "dep", "merge", "--group", "lodash@2.0.0")
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
	exit, stdout, _ := runDep(fake, "dep", "merge", "--group", "lodash@2.0.0", "--require-checks=false")
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
	exit, stdout, _ := runDep(fake, "dep", "merge", "--group", "lodash@2.0.0")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
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
	exit, stdout, _ := runDep(fake, "dep", "merge", "--group", "lodash@2.0.0", "--dry-run")
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
