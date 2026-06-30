// Package manifest defines the gh-infra YAML schema types that the infra
// commands read and emit.
package manifest

const (
	// The schema version stamped on every manifest document. It is
	// nyctereutes-specific, not gh-infra's: the platform is GitLab and the fields
	// and value shapes differ, so the two formats are not interchangeable.
	APIVersion = "nyctereutes/v1"

	// KindRepository tags a document describing a single GitLab project.
	KindRepository = "Repository"
)

// A single GitLab project's desired state as a manifest document.
type Repository struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   RepositoryMetadata `yaml:"metadata"`
	Spec       RepositorySpec     `yaml:"spec"`
}

// Identifies which GitLab project a [Repository] document targets.
type RepositoryMetadata struct {
	Name  string `yaml:"name"`
	Owner string `yaml:"owner"`
}

// The GitLab project basic settings. Pointer fields distinguish "unset" (omitted
// from YAML) from a zero value that is an intentional setting.
type RepositorySpec struct {
	Description *string  `yaml:"description,omitempty"`
	Visibility  *string  `yaml:"visibility,omitempty"`
	Archived    *bool    `yaml:"archived,omitempty"`
	Topics      []string `yaml:"topics,omitempty"`
}
