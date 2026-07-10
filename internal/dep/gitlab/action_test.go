package gitlab

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
)

const methodMerge = "merge"

// The stub errors mirror what glabError hands back for a failed run: the args
// line, then glab's stderr in client-go's "<method> <url>: <code> <message>"
// format, wrapped with the classification sentinel when one applies.
var (
	errStubUnauthorized = fmt.Errorf(
		"%w: glab mr approve 12 -R g/proj: exit status 1\nPOST https://gitlab.com/api/v4/projects/g%%2Fproj/merge_requests/12/approve: 401 {message: 401 Unauthorized}",
		glab.ErrUnauthorized,
	)
	errStub500 = errors.New("glab mr approve: 500 Internal Server Error")
	// A 500 on MR !401: the IID digits sit in the args and URL while nothing
	// marks the failure as unauthorized.
	errStub500ForMR401 = errors.New(
		"glab mr approve 401 -R g/proj: exit status 1\nPOST https://gitlab.com/api/v4/projects/g%2Fproj/merge_requests/401/approve: 500 {message: Internal Server Error}",
	)
)

func TestApproveMRSucceeds(t *testing.T) {
	var gotArgs []string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	})

	if err := NewClient(runner).ApproveMR(context.Background(), "g/proj", 12); err != nil {
		t.Fatalf("ApproveMR() error = %v", err)
	}
	if strings.Join(gotArgs, " ") != "mr approve 12 -R g/proj" {
		t.Errorf("glab args = %v, want [mr approve 12 -R g/proj]", gotArgs)
	}
}

func TestApproveMRTreatsUnauthorizedAsSuccess(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errStubUnauthorized
	})

	if err := NewClient(runner).ApproveMR(context.Background(), "g/proj", 12); err != nil {
		t.Errorf("ApproveMR() with 401 error = %v, want nil (idempotent)", err)
	}
}

func TestApproveMRDoesNotMistakeIIDDigitsForUnauthorized(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errStub500ForMR401
	})

	if err := NewClient(runner).ApproveMR(context.Background(), "g/proj", 401); err == nil {
		t.Error("ApproveMR(!401) with a 500 error = nil, want the failure propagated")
	}
}

func TestApproveMRPropagatesOtherErrors(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errStub500
	})

	if err := NewClient(runner).ApproveMR(context.Background(), "g/proj", 12); err == nil {
		t.Error("ApproveMR() with 500 error = nil, want an error")
	}
}

func TestMergeMRAutoMergeArgs(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		autoMerge bool
		wantFlags []string
	}{
		{name: "squash auto-merge", method: "squash", autoMerge: true, wantFlags: []string{"--squash", "--auto-merge"}},
		{name: "rebase immediate", method: "rebase", autoMerge: false, wantFlags: []string{"--rebase", "--auto-merge=false"}},
		{name: "merge immediate", method: methodMerge, autoMerge: false, wantFlags: []string{"--auto-merge=false"}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var gotArgs []string
			runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
				gotArgs = args
				return nil, nil
			})

			client := NewClient(runner)
			if err := client.MergeMR(context.Background(), "g/proj", 12, testCase.method, testCase.autoMerge); err != nil {
				t.Fatalf("MergeMR() error = %v", err)
			}
			joined := strings.Join(gotArgs, " ")
			if !strings.HasPrefix(joined, "mr merge 12 -R g/proj --yes") {
				t.Errorf("merge args = %q, want prefix %q", joined, "mr merge 12 -R g/proj --yes")
			}
			for _, f := range testCase.wantFlags {
				if !slices.Contains(gotArgs, f) {
					t.Errorf("merge args = %v, want to contain %q", gotArgs, f)
				}
			}
			if testCase.method == methodMerge && slices.Contains(gotArgs, "--squash") {
				t.Errorf("merge args = %v, should not contain --squash for plain merge", gotArgs)
			}
		})
	}
}
