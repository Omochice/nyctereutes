package repository

import (
	"fmt"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The kind of drift a [Change] records.
type ChangeType string

const (
	// Marks a project the manifest declares but GitLab does not have.
	ChangeCreate ChangeType = "create"
	// Marks a field whose live value differs from the declared one.
	ChangeUpdate ChangeType = "update"
)

// Field names shared with the manifest struct tags, kept as constants so the
// scattered copies cannot drift apart.
const (
	fieldDescription = "description"
	fieldVisibility  = "visibility"
	fieldArchived    = "archived"
	fieldTopics      = "topics"
)

// One planned difference between a declared manifest and the live project: a
// whole project to create, or one field to update.
type Change struct {
	Type     ChangeType
	Name     string
	Field    string
	OldValue any
	NewValue any
}

// Renders one plan line. The project header already carries the name, so
// neither a create nor an update line repeats it.
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

// Reports how the live project differs from the declared manifest: a create
// when GitLab lacks the project, otherwise one update per differing declared
// field.
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

// Reports whether left and right hold the same elements ignoring order;
// repeats are significant.
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

// Appends an update unless the manifest is silent about the field (desired is
// nil), in which case the live value is left as-is.
func appendChanged[Value comparable](changes *[]Change, name, field string, desired *Value, current Value) {
	if desired == nil || *desired == current {
		return
	}
	*changes = append(*changes, Change{
		Type: ChangeUpdate, Name: name, Field: field,
		OldValue: current, NewValue: *desired,
	})
}
