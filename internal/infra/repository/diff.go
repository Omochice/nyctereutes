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

// The manifest field names a [Change] reports drift for, named once so the
// scattered literals (including struct tags) do not drift apart.
const (
	fieldDescription = "description"
	fieldVisibility  = "visibility"
	fieldArchived    = "archived"
	fieldTopics      = "topics"
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

// String renders one plan line under the project's own header: a create marks
// the whole project, an update shows the field's live value giving way to the
// declared one. The header already carries the project name, so neither
// repeats it.
func (c Change) String() string {
	switch c.Type {
	case ChangeCreate:
		return "+ new repository"
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
	appendChanged(&changes, name, fieldDescription, desired.Spec.Description, string(current.Description))
	appendChanged(&changes, name, fieldVisibility, desired.Spec.Visibility, manifest.Visibility(current.Visibility))
	appendChanged(&changes, name, fieldArchived, desired.Spec.Archived, current.Archived != nil && *current.Archived)
	// A nil topics list is omitted and left as-is the way a nil pointer is for
	// the scalar fields; an explicit empty list clears the topics. Order
	// carries no meaning.
	if desired.Spec.Topics != nil && !sameStringSet(desired.Spec.Topics, current.Topics) {
		changes = append(changes, Change{
			Type: ChangeUpdate, Name: name, Field: fieldTopics,
			OldValue: current.Topics, NewValue: desired.Spec.Topics,
		})
	}
	return changes
}

// sameStringSet reports whether left and right hold the same elements
// regardless of order, treating repeats as distinct so a genuine multiplicity
// change shows.
func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, s := range left {
		counts[s]++
	}
	for _, s := range right {
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
	if desired == nil || *desired == current {
		return
	}
	*changes = append(*changes, Change{
		Type: ChangeUpdate, Name: name, Field: field,
		OldValue: current, NewValue: *desired,
	})
}
