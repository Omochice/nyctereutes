package repository

import (
	"fmt"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// ChangeType names the kind of drift a [Change] records.
type ChangeType string

const (
	// ChangeCreate marks a project the manifest declares but GitLab does not have.
	ChangeCreate ChangeType = "create"
	// ChangeUpdate marks a field whose live value differs from the declared one.
	ChangeUpdate ChangeType = "update"
)

// Change is one planned difference between a declared manifest and the live
// project: a whole project to create, or one field to update.
type Change struct {
	Type     ChangeType
	Name     string
	Field    string
	OldValue any
	NewValue any
}

// String renders one plan line: a create names the whole project, an update
// shows the field's live value giving way to the declared one.
func (c Change) String() string {
	switch c.Type {
	case ChangeCreate:
		return fmt.Sprintf("+ %s (new repository)", c.Name)
	case ChangeUpdate:
		return fmt.Sprintf("~ %s: %v → %v", c.Field, c.OldValue, c.NewValue)
	default:
		return ""
	}
}

// Diff reports how the live project differs from the declared manifest. A
// project GitLab does not have yields a single create; otherwise each declared
// field that differs from the live value yields an update.
func Diff(desired *manifest.Repository, current *CurrentState) []Change {
	name := desired.Metadata.Owner + "/" + desired.Metadata.Name

	if current.IsNew {
		return []Change{{Type: ChangeCreate, Name: name, Field: "repository", NewValue: name}}
	}

	var changes []Change
	appendChanged(&changes, name, "description", desired.Spec.Description, string(current.Description))
	appendChanged(&changes, name, "visibility", desired.Spec.Visibility, manifest.Visibility(current.Visibility))
	appendChanged(&changes, name, "archived", desired.Spec.Archived, current.Archived != nil && *current.Archived)
	// Topics are a set, and an absent list means "leave as-is" the way a nil
	// pointer does for the scalar fields; order carries no meaning.
	if len(desired.Spec.Topics) > 0 && !sameStringSet(desired.Spec.Topics, current.Topics) {
		changes = append(changes, Change{Type: ChangeUpdate, Name: name, Field: "topics", OldValue: current.Topics, NewValue: desired.Spec.Topics})
	}
	return changes
}

// sameStringSet reports whether a and b hold the same elements regardless of
// order, treating repeats as distinct so a genuine multiplicity change shows.
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

// appendChanged records an update when the manifest declares the field
// (desired is non-nil) and its value differs from the live one. A nil desired
// means the manifest is silent about the field, so the live value is left as-is.
func appendChanged[Value comparable](changes *[]Change, name, field string, desired *Value, current Value) {
	if desired != nil && *desired != current {
		*changes = append(*changes, Change{Type: ChangeUpdate, Name: name, Field: field, OldValue: current, NewValue: *desired})
	}
}
