// Package manifest defines the gh-infra YAML schema types that the infra
// commands read and emit.
package manifest

import (
	"errors"
	"fmt"
	"reflect"

	goyaml "github.com/goccy/go-yaml"
)

// Signals that no supported YAML encoding decodes back to the document.
var errNotRoundTrippable = errors.New("manifest does not survive a yaml round trip")

// Encodes a manifest document to YAML. Every emitter goes through this
// function so the document encoding style has a single owner. Multiline
// values become literal blocks, which requires the LF-normalized values the
// import produces: a literal block cannot carry a bare CR.
//
// Each encoding is verified by decoding it back, because goyaml can emit
// unparseable or lossy documents for representable values: literal blocks
// carry no indentation indicator, so a multiline value whose first line
// starts with whitespace does not parse, and a newline-only value decodes
// back as empty. The attempts run from prettiest to safest; JSON escaping
// represents every string.
func Marshal(doc *Repository) ([]byte, error) {
	// topics carries no omitempty, so nil and empty emit identically as [] and
	// the document cannot express the difference; canonicalizing nil up front
	// keeps the round-trip comparison free of field-specific carve-outs.
	if doc.Spec.Topics == nil {
		canonical := *doc
		canonical.Spec.Topics = []string{}
		doc = &canonical
	}

	attempts := [][]goyaml.EncodeOption{
		{goyaml.UseLiteralStyleIfMultiline(true)},
		{},
		{goyaml.JSON()},
	}
	for _, opts := range attempts {
		out, err := goyaml.MarshalWithOptions(doc, opts...)
		if err != nil {
			return nil, fmt.Errorf("marshal manifest: %w", err)
		}
		if roundTrips(out, doc) {
			return out, nil
		}
	}
	return nil, errNotRoundTrippable
}

// Reports whether out decodes back into a document equal to doc.
func roundTrips(out []byte, doc *Repository) bool {
	var back Repository
	if err := goyaml.Unmarshal(out, &back); err != nil {
		return false
	}
	return reflect.DeepEqual(&back, doc)
}

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

// Who can see a GitLab project: "private", "internal" or "public".
type Visibility string

// How far a project feature is opened up: "disabled", "private" or "enabled".
type AccessLevel string

// An access level for the two toggles (pages, package_registry) that
// additionally accept "public", exposing the feature to everyone even on a
// private project. A separate type keeps "public" rejectable on the other
// feature fields.
type PublicAccessLevel string

// The GitLab project basic settings. Pointer fields distinguish "unset" (omitted
// from YAML) from a zero value that is an intentional setting.
type RepositorySpec struct {
	Description *string     `yaml:"description,omitempty"`
	Visibility  *Visibility `yaml:"visibility,omitempty"`
	// Placed after visibility to match their spot in the settings UI.
	RequestAccessEnabled       *bool `yaml:"request_access_enabled,omitempty"`
	EnforceAuthChecksOnUploads *bool `yaml:"enforce_auth_checks_on_uploads,omitempty"`
	Archived                   *bool `yaml:"archived,omitempty"`
	// No omitempty: an explicit empty topic list must survive export so the YAML
	// fully represents the project's current state.
	Topics        []string `yaml:"topics"`
	DefaultBranch *string  `yaml:"default_branch,omitempty"`
	// Commit message and MR description templates from Settings > Merge
	// requests; GitLab reports null for an unset template, hence pointers.
	MergeCommitTemplate   *string             `yaml:"merge_commit_template,omitempty"`
	SquashCommitTemplate  *string             `yaml:"squash_commit_template,omitempty"`
	MergeRequestsTemplate *string             `yaml:"merge_requests_template,omitempty"`
	Features              *RepositoryFeatures `yaml:"features,omitempty"`
}

// The per-feature access levels of a GitLab project; an unset feature is
// omitted. Fields follow the settings-UI display order rather than the API's,
// so the emitted YAML reads like the settings page.
type RepositoryFeatures struct {
	Issues        *AccessLevel `yaml:"issues,omitempty"`
	Repository    *AccessLevel `yaml:"repository,omitempty"`
	MergeRequests *AccessLevel `yaml:"merge_requests,omitempty"`
	Forking       *AccessLevel `yaml:"forking,omitempty"`
	// GitLab's builds_access_level, exposed under the friendlier "ci" key.
	CICD                  *AccessLevel       `yaml:"ci,omitempty"`
	ContainerRegistry     *AccessLevel       `yaml:"container_registry,omitempty"`
	Analytics             *AccessLevel       `yaml:"analytics,omitempty"`
	Requirements          *AccessLevel       `yaml:"requirements,omitempty"`
	SecurityAndCompliance *AccessLevel       `yaml:"security_and_compliance,omitempty"`
	Wiki                  *AccessLevel       `yaml:"wiki,omitempty"`
	Snippets              *AccessLevel       `yaml:"snippets,omitempty"`
	PackageRegistry       *PublicAccessLevel `yaml:"package_registry,omitempty"`
	ModelExperiments      *AccessLevel       `yaml:"model_experiments,omitempty"`
	ModelRegistry         *AccessLevel       `yaml:"model_registry,omitempty"`
	Pages                 *PublicAccessLevel `yaml:"pages,omitempty"`
	Monitor               *AccessLevel       `yaml:"monitor,omitempty"`
	Environments          *AccessLevel       `yaml:"environments,omitempty"`
	FeatureFlags          *AccessLevel       `yaml:"feature_flags,omitempty"`
	Infrastructure        *AccessLevel       `yaml:"infrastructure,omitempty"`
	Releases              *AccessLevel       `yaml:"releases,omitempty"`
}
