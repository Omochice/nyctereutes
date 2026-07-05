package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// A ProjectWriter fake that records every call so a test can assert the exact
// glab invocation. errAt maps a call index to the error that call returns.
type recordingWriter struct {
	calls []writerCall
	errAt map[int]error
}

type writerCall struct {
	args  []string
	stdin []byte // nil for a plain Run, the body for a RunWithStdin
}

func (w *recordingWriter) Run(_ context.Context, args ...string) ([]byte, error) {
	err := w.errAt[len(w.calls)]
	w.calls = append(w.calls, writerCall{args: args})
	return nil, err
}

func (w *recordingWriter) RunWithStdin(_ context.Context, body []byte, args ...string) ([]byte, error) {
	err := w.errAt[len(w.calls)]
	w.calls = append(w.calls, writerCall{args: args, stdin: body})
	return nil, err
}

func TestApplyPutsScalarField(t *testing.T) {
	writer := &recordingWriter{}
	changes := []Change{
		{Type: ChangeUpdate, Name: "group/proj", Field: fieldDescription, NewValue: "a tool"},
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v, want one successful result", results)
	}
	want := "api projects/group%2Fproj --method PUT -f description=a tool"
	if got := strings.Join(writer.calls[0].args, " "); got != want {
		t.Errorf("glab args = %q, want %q", got, want)
	}
}

func TestApplyPutsIdentityAndBoolFields(t *testing.T) {
	for _, tc := range []struct {
		name     string
		field    string
		newValue any
		wantArg  string
	}{
		{"visibility", fieldVisibility, manifest.Visibility("internal"), "-f visibility=internal"},
		{"default_branch", fieldDefaultBranch, "main", "-f default_branch=main"},
		{"merge_commit_template", fieldMergeCommitTemplate, "%{title}", "-f merge_commit_template=%{title}"},
		{"request_access_enabled", fieldRequestAccessEnabled, true, "-f request_access_enabled=true"},
		{"enforce_auth_checks", fieldEnforceAuthChecksOnUploads, false, "-f enforce_auth_checks_on_uploads=false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &recordingWriter{}
			changes := []Change{{Type: ChangeUpdate, Name: "group/proj", Field: tc.field, NewValue: tc.newValue}}

			results := NewApplier(writer).Apply(context.Background(), changes)

			if len(results) != 1 || results[0].Err != nil {
				t.Fatalf("results = %+v, want one successful result", results)
			}
			want := "api projects/group%2Fproj --method PUT " + tc.wantArg
			if got := strings.Join(writer.calls[0].args, " "); got != want {
				t.Errorf("glab args = %q, want %q", got, want)
			}
		})
	}
}
