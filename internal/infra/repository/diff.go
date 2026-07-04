package repository

import "github.com/Omochice/nyctereutes/internal/infra/manifest"

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
	return changes
}

// appendChanged records an update when the manifest declares the field
// (desired is non-nil) and its value differs from the live one. A nil desired
// means the manifest is silent about the field, so the live value is left as-is.
func appendChanged[Value comparable](changes *[]Change, name, field string, desired *Value, current Value) {
	if desired != nil && *desired != current {
		*changes = append(*changes, Change{Type: ChangeUpdate, Name: name, Field: field, OldValue: current, NewValue: *desired})
	}
}
