package repository

import (
	"testing"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

func TestDiffNewProjectIsCreate(t *testing.T) {
	desired := &manifest.Repository{Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"}}
	current := &CurrentState{Owner: "group", Name: "proj", IsNew: true}

	changes := Diff(desired, current)

	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1 create", len(changes))
	}
	if changes[0].Type != ChangeCreate {
		t.Errorf("Type = %q, want create", changes[0].Type)
	}
	if changes[0].Name != "group/proj" {
		t.Errorf("Name = %q, want group/proj", changes[0].Name)
	}
}

func TestDiffReportsVisibilityChange(t *testing.T) {
	priv := manifest.Visibility("private")
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{Visibility: &priv},
	}
	current := &CurrentState{Owner: "group", Name: "proj", rawProject: rawProject{Visibility: "internal"}}

	changes := Diff(desired, current)

	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(changes))
	}
	c := changes[0]
	if c.Type != ChangeUpdate || c.Field != "visibility" {
		t.Fatalf("change = %+v, want update of visibility", c)
	}
	if c.OldValue != manifest.Visibility("internal") || c.NewValue != manifest.Visibility("private") {
		t.Errorf("values = %v → %v, want internal → private", c.OldValue, c.NewValue)
	}
}
