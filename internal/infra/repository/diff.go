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
	fieldDescription                = "description"
	fieldVisibility                 = "visibility"
	fieldArchived                   = "archived"
	fieldTopics                     = "topics"
	fieldRepository                 = "repository"
	fieldRequestAccessEnabled       = "request_access_enabled"
	fieldEnforceAuthChecksOnUploads = "enforce_auth_checks_on_uploads"
	fieldDefaultBranch              = "default_branch"
	fieldMergeCommitTemplate        = "merge_commit_template"
	fieldSquashCommitTemplate       = "squash_commit_template"
	fieldMergeRequestsTemplate      = "merge_requests_template"
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
		return []Change{{Type: ChangeCreate, Name: name, Field: fieldRepository, NewValue: name}}
	}

	var changes []Change
	spec := desired.Spec
	appendChanged(&changes, name, fieldDescription, spec.Description, string(current.Description))
	appendChanged(&changes, name, fieldVisibility, spec.Visibility, manifest.Visibility(current.Visibility))
	appendChanged(&changes, name, fieldArchived, spec.Archived, boolValue(current.Archived))
	appendChanged(&changes, name, fieldRequestAccessEnabled,
		spec.RequestAccessEnabled, boolValue(current.RequestAccessEnabled))
	appendChanged(&changes, name, fieldEnforceAuthChecksOnUploads,
		spec.EnforceAuthChecksOnUploads, boolValue(current.EnforceAuthChecksOnUploads))
	appendChanged(&changes, name, fieldDefaultBranch, spec.DefaultBranch, string(current.DefaultBranch))
	appendChanged(&changes, name, fieldMergeCommitTemplate,
		spec.MergeCommitTemplate, textValue(current.MergeCommitTemplate))
	appendChanged(&changes, name, fieldSquashCommitTemplate,
		spec.SquashCommitTemplate, textValue(current.SquashCommitTemplate))
	appendChanged(&changes, name, fieldMergeRequestsTemplate,
		spec.MergeRequestsTemplate, textValue(current.MergeRequestsTemplate))
	// A nil topics list is omitted and left as-is the way a nil pointer is for
	// the scalar fields; an explicit empty list clears the topics. Order
	// carries no meaning.
	if spec.Topics != nil && !sameStringSet(spec.Topics, current.Topics) {
		changes = append(changes, Change{
			Type: ChangeUpdate, Name: name, Field: fieldTopics,
			OldValue: current.Topics, NewValue: spec.Topics,
		})
	}
	// A nil features block manages no feature, so its access levels are left
	// as-is; a declared block still leaves its own nil fields untouched.
	if spec.Features != nil {
		diffFeatures(&changes, name, spec.Features, current)
	}
	return changes
}

// Records drift for each declared per-feature access level, under a
// "features.<key>" field mirroring the manifest path.
func diffFeatures(changes *[]Change, name string, desired *manifest.RepositoryFeatures, current *CurrentState) {
	level := func(field string, want *manifest.AccessLevel, live reported) {
		appendChanged(changes, name, field, want, manifest.AccessLevel(live))
	}
	public := func(field string, want *manifest.PublicAccessLevel, live reported) {
		appendChanged(changes, name, field, want, manifest.PublicAccessLevel(live))
	}
	level("features.issues", desired.Issues, current.IssuesAccessLevel)
	level("features.repository", desired.Repository, current.RepositoryAccessLevel)
	level("features.merge_requests", desired.MergeRequests, current.MergeRequestsAccessLevel)
	level("features.forking", desired.Forking, current.ForkingAccessLevel)
	level("features.ci", desired.CICD, current.BuildsAccessLevel)
	level("features.container_registry", desired.ContainerRegistry, current.ContainerRegistryAccessLevel)
	level("features.analytics", desired.Analytics, current.AnalyticsAccessLevel)
	level("features.requirements", desired.Requirements, current.RequirementsAccessLevel)
	level("features.security_and_compliance", desired.SecurityAndCompliance, current.SecurityAndComplianceAccessLevel)
	level("features.wiki", desired.Wiki, current.WikiAccessLevel)
	level("features.snippets", desired.Snippets, current.SnippetsAccessLevel)
	public("features.package_registry", desired.PackageRegistry, current.PackageRegistryAccessLevel)
	level("features.model_experiments", desired.ModelExperiments, current.ModelExperimentsAccessLevel)
	level("features.model_registry", desired.ModelRegistry, current.ModelRegistryAccessLevel)
	public("features.pages", desired.Pages, current.PagesAccessLevel)
	level("features.monitor", desired.Monitor, current.MonitorAccessLevel)
	level("features.environments", desired.Environments, current.EnvironmentsAccessLevel)
	level("features.feature_flags", desired.FeatureFlags, current.FeatureFlagsAccessLevel)
	level("features.infrastructure", desired.Infrastructure, current.InfrastructureAccessLevel)
	level("features.releases", desired.Releases, current.ReleasesAccessLevel)
}

// A nil pointer is the live value GitLab did not report; it counts as the zero
// value so a manifest declaring that zero is not seen as drift.
func boolValue(b *bool) bool {
	return b != nil && *b
}

func textValue(t *freeText) string {
	if t == nil {
		return ""
	}
	return string(*t)
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
