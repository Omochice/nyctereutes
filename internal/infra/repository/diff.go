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

	return nil
}
