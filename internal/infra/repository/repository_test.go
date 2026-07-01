package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

const sampleDescription = "a tool"

var errGlab404 = errors.New("glab api projects/x: exit status 1\n404 Project Not Found")

func TestFetchRepositoryParsesSettings(t *testing.T) {
	var gotArgs []string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{"description":"a tool","visibility":"private","topics":["go","cli"],"archived":true}`), nil
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), "group/sub", "proj")
	if err != nil {
		t.Fatalf("FetchRepository: %v", err)
	}

	if want := []string{"api", "projects/group%2Fsub%2Fproj"}; strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("glab args = %v, want %v", gotArgs, want)
	}
	if state.IsNew {
		t.Errorf("IsNew = true, want false for an existing project")
	}
	if state.Description != sampleDescription || state.Visibility != "private" || !state.Archived {
		t.Errorf("state = %+v, want description/visibility/archived populated", state)
	}
	if strings.Join(state.Topics, ",") != "go,cli" {
		t.Errorf("topics = %v, want [go cli]", state.Topics)
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
		Owner:       "group",
		Name:        "proj",
		Description: sampleDescription,
		Archived:    true,
		Visibility:  "private",
		Topics:      []string{"go"},
	}

	doc := ToManifest(state)

	if doc.APIVersion != manifest.APIVersion {
		t.Errorf("apiVersion = %q, want %q", doc.APIVersion, manifest.APIVersion)
	}
	if doc.Kind != manifest.KindRepository {
		t.Errorf("kind = %q, want %q", doc.Kind, manifest.KindRepository)
	}
	if doc.Metadata.Name != "proj" || doc.Metadata.Owner != "group" {
		t.Errorf("metadata = %+v, want name=proj owner=group", doc.Metadata)
	}
	if doc.Spec.Description == nil || *doc.Spec.Description != sampleDescription {
		t.Errorf("spec.description = %v, want %q", doc.Spec.Description, sampleDescription)
	}
	if doc.Spec.Archived == nil || !*doc.Spec.Archived {
		t.Errorf("spec.archived = %v, want true", doc.Spec.Archived)
	}
}
