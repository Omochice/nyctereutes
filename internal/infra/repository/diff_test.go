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

func TestDiffReportsDescriptionChange(t *testing.T) {
	want := "new text"
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{Description: &want},
	}
	current := &CurrentState{rawProject: rawProject{Description: "old text"}}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "description" {
		t.Fatalf("changes = %+v, want one description update", changes)
	}
	if changes[0].OldValue != "old text" || changes[0].NewValue != "new text" {
		t.Errorf("values = %v → %v, want old text → new text", changes[0].OldValue, changes[0].NewValue)
	}
}

func TestDiffReportsArchivedChange(t *testing.T) {
	archived := true
	liveFalse := false
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{Archived: &archived},
	}
	current := &CurrentState{rawProject: rawProject{Archived: &liveFalse}}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "archived" {
		t.Fatalf("changes = %+v, want one archived update", changes)
	}
	if changes[0].OldValue != false || changes[0].NewValue != true {
		t.Errorf("values = %v → %v, want false → true", changes[0].OldValue, changes[0].NewValue)
	}
}

// A manifest that declares no spec fields manages nothing, so a live project
// that differs in those fields must still yield no drift.
func TestDiffLeavesUndeclaredFieldsUnchanged(t *testing.T) {
	desired := &manifest.Repository{Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"}}
	live := false
	current := &CurrentState{rawProject: rawProject{
		Description: "live text",
		Visibility:  "public",
		Archived:    &live,
		Topics:      []string{"go"},
	}}

	changes := Diff(desired, current)

	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none for a silent manifest", changes)
	}
}

func TestDiffReportsNoChangeWhenDeclaredMatchesLive(t *testing.T) {
	desc := "same"
	vis := manifest.Visibility("private")
	arch := false
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec: manifest.RepositorySpec{
			Description: &desc,
			Visibility:  &vis,
			Archived:    &arch,
			Topics:      []string{"go", "cli"},
		},
	}
	current := &CurrentState{rawProject: rawProject{
		Description: "same",
		Visibility:  "private",
		Archived:    &arch,
		Topics:      []string{"cli", "go"},
	}}

	if changes := Diff(desired, current); len(changes) != 0 {
		t.Errorf("changes = %+v, want none when declared matches live", changes)
	}
}

func TestDiffComparesTopicsAsSet(t *testing.T) {
	t.Run("same set in another order is no change", func(t *testing.T) {
		desired := &manifest.Repository{
			Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
			Spec:     manifest.RepositorySpec{Topics: []string{"go", "cli"}},
		}
		current := &CurrentState{rawProject: rawProject{Topics: []string{"cli", "go"}}}

		if changes := Diff(desired, current); len(changes) != 0 {
			t.Errorf("changes = %+v, want none for the same topic set", changes)
		}
	})

	t.Run("different set is an update", func(t *testing.T) {
		desired := &manifest.Repository{
			Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
			Spec:     manifest.RepositorySpec{Topics: []string{"go", "rust"}},
		}
		current := &CurrentState{rawProject: rawProject{Topics: []string{"go"}}}

		changes := Diff(desired, current)

		if len(changes) != 1 || changes[0].Field != "topics" {
			t.Fatalf("changes = %+v, want one topics update", changes)
		}
	})
}
