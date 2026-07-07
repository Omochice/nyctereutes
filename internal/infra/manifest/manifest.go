// Package manifest defines the gh-infra YAML schema types that the infra
// commands read and emit.
package manifest

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	goyaml "github.com/goccy/go-yaml"
)

// Signals that no supported YAML encoding decodes back to the document.
var errNotRoundTrippable = errors.New("manifest does not survive a yaml round trip")

// Signals an encoding that parses but decodes into a different document.
var errLossyEncoding = errors.New("decoded document differs from the source")

// Signals a manifest enum value outside its allowed set.
var errInvalidValue = errors.New("invalid value")

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
	var lastErr error
	for _, opts := range attempts {
		out, err := goyaml.MarshalWithOptions(doc, opts...)
		if err != nil {
			return nil, fmt.Errorf("marshal manifest: %w", err)
		}
		if err := roundTrips(out, doc); err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: %w", errNotRoundTrippable, lastErr)
}

// Reports why out does not decode back into a document equal to doc, or nil
// when it does. The decode error must be preserved, not reduced to a bool: it
// is the only message that names the field and value that cannot round-trip
// (an enum value outside the schema, for example), so swallowing it would
// leave an import failure with no actionable cause.
func roundTrips(out []byte, doc *Repository) error {
	var back Repository
	if err := goyaml.Unmarshal(out, &back); err != nil {
		return fmt.Errorf("decode emitted yaml: %w", err)
	}
	if !reflect.DeepEqual(&back, doc) {
		return errLossyEncoding
	}
	return nil
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

const (
	valueDisabled = "disabled"
	valueEnabled  = "enabled"
	valueInternal = "internal"
	valuePrivate  = "private"
	valuePublic   = "public"
)

// Who can see a GitLab project: "private", "internal" or "public".
type Visibility string

// Rejects values outside the visibility set at decode time.
func (visibility *Visibility) UnmarshalYAML(data []byte) error {
	value, err := enumValue(data, "visibility", valuePrivate, valueInternal, valuePublic)
	if err != nil {
		return err
	}
	*visibility = Visibility(value)
	return nil
}

// How far a project feature is opened up: "disabled", "private" or "enabled".
type AccessLevel string

// Rejects values outside the access-level set at decode time; notably
// "public", which only the public-capable toggles accept.
func (level *AccessLevel) UnmarshalYAML(data []byte) error {
	value, err := enumValue(data, "access level", valueDisabled, valuePrivate, valueEnabled)
	if err != nil {
		return err
	}
	*level = AccessLevel(value)
	return nil
}

// An access level for the two toggles (pages, package_registry) that
// additionally accept "public", exposing the feature to everyone even on a
// private project. A separate type keeps "public" rejectable on the other
// feature fields.
type PublicAccessLevel string

// Rejects values outside the public-capable access-level set at decode time.
func (level *PublicAccessLevel) UnmarshalYAML(data []byte) error {
	value, err := enumValue(data, "access level", valueDisabled, valuePrivate, valueEnabled, valuePublic)
	if err != nil {
		return err
	}
	*level = PublicAccessLevel(value)
	return nil
}

const (
	valueMerge       = "merge"
	valueRebaseMerge = "rebase_merge"
	valueFastForward = "ff"
)

// How GitLab lands an accepted merge request on the target branch: "merge" (a
// merge commit), "rebase_merge" (a merge commit atop a rebased, semi-linear
// history) or "ff" (fast-forward, adding no merge commit).
type MergeMethod string

// Rejects values outside the merge-method set at decode time.
func (method *MergeMethod) UnmarshalYAML(data []byte) error {
	value, err := enumValue(data, "merge method", valueMerge, valueRebaseMerge, valueFastForward)
	if err != nil {
		return err
	}
	*method = MergeMethod(value)
	return nil
}

// Decodes a scalar enum value, rejecting anything outside allowed with an
// error that lists the allowed values, so a typo in a hand-edited manifest
// is self-explanatory.
func enumValue(data []byte, kind string, allowed ...string) (string, error) {
	var value string
	if err := goyaml.Unmarshal(data, &value); err != nil {
		return "", fmt.Errorf("decode %s: %w", kind, err)
	}
	if !slices.Contains(allowed, value) {
		return "", fmt.Errorf("%w: %s %q must be one of: %s",
			errInvalidValue, kind, value, strings.Join(allowed, ", "))
	}
	return value, nil
}

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
	// GitLab's merge_method, from Settings > Merge requests > Merge method:
	// whether an accepted MR lands as a merge commit, a semi-linear merge or a
	// fast-forward.
	MergeMethod *MergeMethod `yaml:"merge_method,omitempty"`
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
