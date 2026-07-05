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

func TestApplyMapsFeatureFieldsToAccessLevelParams(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		field   string
		wantArg string
	}{
		{"issues", "features.issues", "-f issues_access_level=enabled"},
		{"container_registry", "features.container_registry", "-f container_registry_access_level=enabled"},
		{"ci maps to builds", "features.ci", "-f builds_access_level=enabled"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			writer := &recordingWriter{}
			changes := []Change{
				{Type: ChangeUpdate, Name: "group/proj", Field: testCase.field, NewValue: manifest.AccessLevel("enabled")},
			}

			results := NewApplier(writer).Apply(context.Background(), changes)

			if len(results) != 1 || results[0].Err != nil {
				t.Fatalf("results = %+v, want one successful result", results)
			}
			want := "api projects/group%2Fproj --method PUT " + testCase.wantArg
			if got := strings.Join(writer.calls[0].args, " "); got != want {
				t.Errorf("glab args = %q, want %q", got, want)
			}
		})
	}
}

func TestApplyReplacesTopicsViaJSONStdin(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		topics   []string
		wantBody string
	}{
		{"replace", []string{"go", "cli"}, `{"topics":["go","cli"]}`},
		{"clear", []string{}, `{"topics":[]}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			writer := &recordingWriter{}
			changes := []Change{{Type: ChangeUpdate, Name: "group/proj", Field: fieldTopics, NewValue: testCase.topics}}

			results := NewApplier(writer).Apply(context.Background(), changes)

			if len(results) != 1 || results[0].Err != nil {
				t.Fatalf("results = %+v, want one successful result", results)
			}
			wantArgs := "api projects/group%2Fproj --method PUT --header Content-Type: application/json --input -"
			if got := strings.Join(writer.calls[0].args, " "); got != wantArgs {
				t.Errorf("glab args = %q, want %q", got, wantArgs)
			}
			if got := string(writer.calls[0].stdin); got != testCase.wantBody {
				t.Errorf("stdin = %q, want %q", got, testCase.wantBody)
			}
		})
	}
}

func TestApplyArchivesThroughDedicatedEndpoint(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		archived bool
		wantPath string
	}{
		{"archive", true, "projects/group%2Fproj/archive"},
		{"unarchive", false, "projects/group%2Fproj/unarchive"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			writer := &recordingWriter{}
			changes := []Change{{Type: ChangeUpdate, Name: "group/proj", Field: fieldArchived, NewValue: testCase.archived}}

			results := NewApplier(writer).Apply(context.Background(), changes)

			if len(results) != 1 || results[0].Err != nil {
				t.Fatalf("results = %+v, want one successful result", results)
			}
			want := "api " + testCase.wantPath + " --method POST"
			if got := strings.Join(writer.calls[0].args, " "); got != want {
				t.Errorf("glab args = %q, want %q", got, want)
			}
		})
	}
}

func TestApplyPutsIdentityAndBoolFields(t *testing.T) {
	for _, testCase := range []struct {
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
		t.Run(testCase.name, func(t *testing.T) {
			writer := &recordingWriter{}
			changes := []Change{{Type: ChangeUpdate, Name: "group/proj", Field: testCase.field, NewValue: testCase.newValue}}

			results := NewApplier(writer).Apply(context.Background(), changes)

			if len(results) != 1 || results[0].Err != nil {
				t.Fatalf("results = %+v, want one successful result", results)
			}
			want := "api projects/group%2Fproj --method PUT " + testCase.wantArg
			if got := strings.Join(writer.calls[0].args, " "); got != want {
				t.Errorf("glab args = %q, want %q", got, want)
			}
		})
	}
}
