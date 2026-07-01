package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

const (
	sampleDescription = "a tool"
	visibilityPrivate = "private"
	levelEnabled      = "enabled"
	levelDisabled     = "disabled"
)

const sampleProjectJSON = `{"description":"a tool","visibility":"private","topics":["go","cli"],"archived":true,` +
	`"issues_access_level":"enabled","merge_requests_access_level":"private","wiki_access_level":"disabled",` +
	`"builds_access_level":"enabled","snippets_access_level":"enabled","container_registry_access_level":"private"}`

var errGlab404 = errors.New("glab api projects/x: exit status 1\n404 Project Not Found")

// wantPtr fails the test unless got points to want.
func wantPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %q", name, got, want)
	}
}

func TestFetchRepositoryParsesSettings(t *testing.T) {
	var gotArgs []string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(sampleProjectJSON), nil
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), "group/sub", "proj")
	if err != nil {
		t.Fatalf("FetchRepository: %v", err)
	}
	if state.IsNew {
		t.Errorf("IsNew = true, want false for an existing project")
	}
	if !state.Archived {
		t.Errorf("archived = false, want true")
	}
	if want := "api projects/group%2Fsub%2Fproj"; strings.Join(gotArgs, " ") != want {
		t.Errorf("glab args = %v, want %q", gotArgs, want)
	}

	for _, check := range []struct{ name, got, want string }{
		{"description", state.Description, sampleDescription},
		{"visibility", state.Visibility, visibilityPrivate},
		{"topics", strings.Join(state.Topics, ","), "go,cli"},
		{"issues", state.IssuesAccessLevel, levelEnabled},
		{"wiki", state.WikiAccessLevel, levelDisabled},
		{"builds", state.BuildsAccessLevel, levelEnabled},
		{"container_registry", state.ContainerRegistryAccessLevel, visibilityPrivate},
	} {
		if check.got != check.want {
			t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
		}
	}
}

func TestFetchRepositoryNotFoundIsNew(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errGlab404
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), "group", "missing")
	if err != nil {
		t.Fatalf("FetchRepository should not error on 404, got %v", err)
	}
	if !state.IsNew {
		t.Errorf("IsNew = false, want true for a missing project")
	}
}

func TestToManifest(t *testing.T) {
	state := &CurrentState{
		Owner:             "group",
		Name:              "proj",
		Description:       sampleDescription,
		Archived:          true,
		Visibility:        visibilityPrivate,
		Topics:            []string{"go"},
		IssuesAccessLevel: levelEnabled,
		WikiAccessLevel:   levelDisabled,
	}

	doc := ToManifest(state)

	if doc.APIVersion != manifest.APIVersion {
		t.Errorf("apiVersion = %q, want %q", doc.APIVersion, manifest.APIVersion)
	}
	if doc.Kind != manifest.KindRepository {
		t.Errorf("kind = %q, want %q", doc.Kind, manifest.KindRepository)
	}
	if (doc.Metadata != manifest.RepositoryMetadata{Name: "proj", Owner: "group"}) {
		t.Errorf("metadata = %+v, want name=proj owner=group", doc.Metadata)
	}
	wantPtr(t, "spec.description", doc.Spec.Description, sampleDescription)
	if doc.Spec.Archived == nil || !*doc.Spec.Archived {
		t.Errorf("spec.archived = %v, want true", doc.Spec.Archived)
	}
	if doc.Spec.Features == nil {
		t.Fatal("spec.features = nil, want populated")
	}
	wantPtr(t, "features.issues", doc.Spec.Features.Issues, levelEnabled)
	wantPtr(t, "features.wiki", doc.Spec.Features.Wiki, levelDisabled)
	// An access level not returned by GitLab is omitted rather than emitted empty.
	if doc.Spec.Features.Snippets != nil {
		t.Errorf("features.snippets = %v, want nil when the access level is absent", doc.Spec.Features.Snippets)
	}
}

func TestToManifestOmitsFeaturesWhenAllEmpty(t *testing.T) {
	doc := ToManifest(&CurrentState{Owner: "group", Name: "proj", Visibility: visibilityPrivate})
	if doc.Spec.Features != nil {
		t.Errorf("spec.features = %v, want nil when no access level was reported", doc.Spec.Features)
	}
}
