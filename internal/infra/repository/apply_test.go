package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
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

// A stand-in glab failure so a test can force one call to fail.
var errBoom = errors.New("boom")

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

func TestApplyRunsEveryChangeAndReportsEachOutcome(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{1: errBoom}}
	changes := []Change{
		{Type: ChangeUpdate, Name: "group/sub/proj", Field: fieldDescription, NewValue: "x"},
		{Type: ChangeUpdate, Name: "group/sub/proj", Field: fieldVisibility, NewValue: manifest.Visibility("private")},
		{Type: ChangeUpdate, Name: "group/sub/proj", Field: fieldDefaultBranch, NewValue: "main"},
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Errorf("edge results = %v/%v, want both nil", results[0].Err, results[2].Err)
	}
	if results[1].Err == nil {
		t.Errorf("middle result Err = nil, want the failure")
	}
	if len(writer.calls) != 3 {
		t.Errorf("calls = %d, want 3 (every change runs despite the failure)", len(writer.calls))
	}
	if got := writer.calls[0].args[1]; got != "projects/group%2Fsub%2Fproj" {
		t.Errorf("nested path = %q, want projects/group%%2Fsub%%2Fproj", got)
	}
	if results[0].Change.Field != fieldDescription || results[2].Change.Field != fieldDefaultBranch {
		t.Errorf("result order not preserved: %q, %q", results[0].Change.Field, results[2].Change.Field)
	}
}

func TestApplyFeatureFailureNamesPlanField(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}
	changes := []Change{
		{Type: ChangeUpdate, Name: "group/proj", Field: "features.issues", NewValue: manifest.AccessLevel("enabled")},
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results = %+v, want one failed result", results)
	}
	if got := results[0].Err.Error(); !strings.Contains(got, "features.issues") {
		t.Errorf("error = %q, want it to name the plan field features.issues", got)
	}
	if got := results[0].Err.Error(); strings.Contains(got, "issues_access_level") {
		t.Errorf("error = %q, should not leak the API param name", got)
	}
}

// A classified write failure keeps the sentinel in the chain, so errors.Is
// still identifies the cause, and prepends a human hint that says how to act
// on that class of failure. One case per sentinel so a swapped hint cannot
// pass.
func TestApplyClassifiesWriteFailures(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		cause    error
		wantHint string
	}{
		{"forbidden", fmt.Errorf("%w: 403 Forbidden", glab.ErrForbidden), "permission"},
		{"not found", fmt.Errorf("%w: 404 Not Found", glab.ErrNotFound), "not found"},
		{"validation", fmt.Errorf("%w: 400 Bad Request", glab.ErrValidation), "reject"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			writer := &recordingWriter{errAt: map[int]error{0: testCase.cause}}
			changes := []Change{
				{Type: ChangeUpdate, Name: "group/proj", Field: fieldDescription, NewValue: "x"},
			}

			results := NewApplier(writer).Apply(context.Background(), changes)

			if len(results) != 1 || results[0].Err == nil {
				t.Fatalf("results = %+v, want one failed result", results)
			}
			if !errors.Is(results[0].Err, testCase.cause) {
				t.Errorf("error = %v, want it to keep the sentinel cause in the chain", results[0].Err)
			}
			if got := results[0].Err.Error(); !strings.Contains(got, testCase.wantHint) {
				t.Errorf("error = %q, want a hint containing %q", got, testCase.wantHint)
			}
			if got := results[0].Err.Error(); !strings.Contains(got, "description") ||
				!strings.Contains(got, "group/proj") {
				t.Errorf("error = %q, want it to still name the field and project", got)
			}
		})
	}
}

// An unclassified write failure carries no hint, only the field, project, and
// the underlying error, so the generic form is unchanged for non-API errors.
func TestApplyUnclassifiedFailureStaysGeneric(t *testing.T) {
	writer := &recordingWriter{errAt: map[int]error{0: errBoom}}
	changes := []Change{
		{Type: ChangeUpdate, Name: "group/proj", Field: fieldDescription, NewValue: "x"},
	}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results = %+v, want one failed result", results)
	}
	if want := "apply description on group/proj: boom"; results[0].Err.Error() != want {
		t.Errorf("error = %q, want %q", results[0].Err.Error(), want)
	}
}

func TestApplyReportsCreateAsUnsupported(t *testing.T) {
	writer := &recordingWriter{}
	changes := []Change{{Type: ChangeCreate, Name: "group/newproj", Field: fieldRepository, NewValue: "group/newproj"}}

	results := NewApplier(writer).Apply(context.Background(), changes)

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results = %+v, want one failed result", results)
	}
	if len(writer.calls) != 0 {
		t.Errorf("calls = %d, want 0 (create must not write)", len(writer.calls))
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
		{"merge_method", fieldMergeMethod, manifest.MergeMethod("ff"), "-f merge_method=ff"},
		{"merge_commit_template", fieldMergeCommitTemplate, "%{title}", "-f merge_commit_template=%{title}"},
		{"request_access_enabled", fieldRequestAccessEnabled, true, "-f request_access_enabled=true"},
		{"enforce_auth_checks", fieldEnforceAuthChecksOnUploads, false, "-f enforce_auth_checks_on_uploads=false"},
		{"pipeline_succeeds", fieldOnlyAllowMergeIfPipelineSucceeds, true, "-f only_allow_merge_if_pipeline_succeeds=true"},
		{"skipped_pipeline", fieldAllowMergeOnSkippedPipeline, false, "-f allow_merge_on_skipped_pipeline=false"},
		{
			"discussions_resolved", fieldOnlyAllowMergeIfAllDiscussionsAreResolved, true,
			"-f only_allow_merge_if_all_discussions_are_resolved=true",
		},
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
