package repository

import (
	"fmt"
	"strings"
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

func TestDiffReportsCICatalogChange(t *testing.T) {
	want := true
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{Description: new("a tool"), CICatalog: &want},
	}
	current := &CurrentState{
		rawProject:      rawProject{Description: "a tool"},
		CatalogResource: false,
	}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "ci_catalog" {
		t.Fatalf("changes = %+v, want one ci_catalog update", changes)
	}
	if changes[0].OldValue != false || changes[0].NewValue != true {
		t.Errorf("values = %v → %v, want false → true", changes[0].OldValue, changes[0].NewValue)
	}
}

// A manifest silent about ci_catalog manages neither state, so a live catalog
// resource yields no drift when the field is omitted.
func TestDiffLeavesCICatalogUnchangedWhenUndeclared(t *testing.T) {
	desired := &manifest.Repository{Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"}}
	current := &CurrentState{CatalogResource: true}

	if changes := Diff(desired, current); len(changes) != 0 {
		t.Errorf("changes = %+v, want none when ci_catalog is undeclared", changes)
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

	// An explicit empty list means "clear the topics", distinct from an
	// omitted (nil) list that leaves them as-is.
	t.Run("clearing topics is an update", func(t *testing.T) {
		desired := &manifest.Repository{
			Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
			Spec:     manifest.RepositorySpec{Topics: []string{}},
		}
		current := &CurrentState{rawProject: rawProject{Topics: []string{"go"}}}

		changes := Diff(desired, current)

		if len(changes) != 1 || changes[0].Field != "topics" {
			t.Fatalf("changes = %+v, want one topics update", changes)
		}
	})
}

func TestDiffReportsBooleanChanges(t *testing.T) {
	enabled := true
	disabled := false
	cases := []struct {
		name  string
		spec  manifest.RepositorySpec
		state rawProject
	}{
		{
			name:  "request_access_enabled",
			spec:  manifest.RepositorySpec{RequestAccessEnabled: &enabled},
			state: rawProject{RequestAccessEnabled: &disabled},
		},
		{
			name:  "enforce_auth_checks_on_uploads",
			spec:  manifest.RepositorySpec{EnforceAuthChecksOnUploads: &enabled},
			state: rawProject{EnforceAuthChecksOnUploads: &disabled},
		},
		{
			name:  "only_allow_merge_if_pipeline_succeeds",
			spec:  manifest.RepositorySpec{OnlyAllowMergeIfPipelineSucceeds: &enabled},
			state: rawProject{OnlyAllowMergeIfPipelineSucceeds: &disabled},
		},
		{
			name:  "allow_merge_on_skipped_pipeline",
			spec:  manifest.RepositorySpec{AllowMergeOnSkippedPipeline: &enabled},
			state: rawProject{AllowMergeOnSkippedPipeline: &disabled},
		},
		{
			name:  "only_allow_merge_if_all_discussions_are_resolved",
			spec:  manifest.RepositorySpec{OnlyAllowMergeIfAllDiscussionsAreResolved: &enabled},
			state: rawProject{OnlyAllowMergeIfAllDiscussionsAreResolved: &disabled},
		},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			desired := &manifest.Repository{
				Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
				Spec:     attr.spec,
			}
			changes := Diff(desired, &CurrentState{rawProject: attr.state})
			if len(changes) != 1 || changes[0].Field != attr.name {
				t.Fatalf("changes = %+v, want one %s update", changes, attr.name)
			}
			if changes[0].OldValue != false || changes[0].NewValue != true {
				t.Errorf("values = %v → %v, want false → true", changes[0].OldValue, changes[0].NewValue)
			}
		})
	}
}

func TestDiffReportsDefaultBranchChange(t *testing.T) {
	main := "main"
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{DefaultBranch: &main},
	}
	current := &CurrentState{rawProject: rawProject{DefaultBranch: "master"}}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "default_branch" {
		t.Fatalf("changes = %+v, want one default_branch update", changes)
	}
	if changes[0].OldValue != "master" || changes[0].NewValue != "main" {
		t.Errorf("values = %v → %v, want master → main", changes[0].OldValue, changes[0].NewValue)
	}
}

func TestDiffReportsMergeMethodChange(t *testing.T) {
	ff := manifest.MergeMethod("ff")
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{MergeMethod: &ff},
	}
	current := &CurrentState{rawProject: rawProject{MergeMethod: "merge"}}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "merge_method" {
		t.Fatalf("changes = %+v, want one merge_method update", changes)
	}
	if changes[0].OldValue != manifest.MergeMethod("merge") || changes[0].NewValue != manifest.MergeMethod("ff") {
		t.Errorf("values = %v → %v, want merge → ff", changes[0].OldValue, changes[0].NewValue)
	}
}

func TestDiffReportsTemplateChanges(t *testing.T) {
	want := "new"
	live := freeText("old")
	cases := []struct {
		name  string
		spec  manifest.RepositorySpec
		state rawProject
	}{
		{
			name:  "merge_commit_template",
			spec:  manifest.RepositorySpec{MergeCommitTemplate: &want},
			state: rawProject{MergeCommitTemplate: &live},
		},
		{
			name:  "squash_commit_template",
			spec:  manifest.RepositorySpec{SquashCommitTemplate: &want},
			state: rawProject{SquashCommitTemplate: &live},
		},
		{
			name:  "merge_requests_template",
			spec:  manifest.RepositorySpec{MergeRequestsTemplate: &want},
			state: rawProject{MergeRequestsTemplate: &live},
		},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			desired := &manifest.Repository{
				Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
				Spec:     attr.spec,
			}
			changes := Diff(desired, &CurrentState{rawProject: attr.state})
			if len(changes) != 1 || changes[0].Field != attr.name {
				t.Fatalf("changes = %+v, want one %s update", changes, attr.name)
			}
			if changes[0].OldValue != "old" || changes[0].NewValue != "new" {
				t.Errorf("values = %v → %v, want old → new", changes[0].OldValue, changes[0].NewValue)
			}
		})
	}
}

// An unreported bool or template (nil live pointer) counts as the zero value,
// so a manifest declaring that zero is not seen as drift.
func TestDiffTreatsUnreportedAsZero(t *testing.T) {
	off := false
	empty := ""
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec: manifest.RepositorySpec{
			RequestAccessEnabled: &off,
			MergeCommitTemplate:  &empty,
		},
	}
	current := &CurrentState{rawProject: rawProject{
		RequestAccessEnabled: nil,
		MergeCommitTemplate:  nil,
	}}

	if changes := Diff(desired, current); len(changes) != 0 {
		t.Errorf("changes = %+v, want none when declared zero matches unreported live", changes)
	}
}

// A manifest that omits the features block manages no feature, so live access
// levels must not surface as drift.
func TestDiffIgnoresAbsentFeaturesBlock(t *testing.T) {
	desired := &manifest.Repository{Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"}}
	current := &CurrentState{rawProject: rawProject{
		IssuesAccessLevel: "disabled",
		WikiAccessLevel:   "enabled",
	}}

	if changes := Diff(desired, current); len(changes) != 0 {
		t.Errorf("changes = %+v, want none when the features block is absent", changes)
	}
}

// everyFeatureDeclared asks for "enabled" on every feature so a single Diff
// exercises all the feature wiring at once.
func everyFeatureDeclared() *manifest.RepositoryFeatures {
	level := manifest.AccessLevel("enabled")
	public := manifest.PublicAccessLevel("enabled")
	return &manifest.RepositoryFeatures{
		Issues:                &level,
		Repository:            &level,
		MergeRequests:         &level,
		Forking:               &level,
		CICD:                  &level,
		ContainerRegistry:     &level,
		Analytics:             &level,
		Requirements:          &level,
		SecurityAndCompliance: &level,
		Wiki:                  &level,
		Snippets:              &level,
		PackageRegistry:       &public,
		ModelExperiments:      &level,
		ModelRegistry:         &level,
		Pages:                 &public,
		Monitor:               &level,
		Environments:          &level,
		FeatureFlags:          &level,
		Infrastructure:        &level,
		Releases:              &level,
	}
}

// everyFeatureLive gives each live access level a sentinel equal to its
// manifest feature key, so a change's old value reveals which field was read.
func everyFeatureLive() rawProject {
	return rawProject{
		IssuesAccessLevel:                "issues",
		RepositoryAccessLevel:            "repository",
		MergeRequestsAccessLevel:         "merge_requests",
		ForkingAccessLevel:               "forking",
		BuildsAccessLevel:                "ci",
		ContainerRegistryAccessLevel:     "container_registry",
		AnalyticsAccessLevel:             "analytics",
		RequirementsAccessLevel:          "requirements",
		SecurityAndComplianceAccessLevel: "security_and_compliance",
		WikiAccessLevel:                  "wiki",
		SnippetsAccessLevel:              "snippets",
		PackageRegistryAccessLevel:       "package_registry",
		ModelExperimentsAccessLevel:      "model_experiments",
		ModelRegistryAccessLevel:         "model_registry",
		PagesAccessLevel:                 "pages",
		MonitorAccessLevel:               "monitor",
		EnvironmentsAccessLevel:          "environments",
		FeatureFlagsAccessLevel:          "feature_flags",
		InfrastructureAccessLevel:        "infrastructure",
		ReleasesAccessLevel:              "releases",
	}
}

// Every feature key must diff against its own live access level: with the
// sentinels above, each change's old value must equal the key after
// "features.". Distinct keys are counted so a duplicate cannot silently stand
// in for an omitted one while the total still looks right.
func TestDiffMapsEveryFeature(t *testing.T) {
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec:     manifest.RepositorySpec{Features: everyFeatureDeclared()},
	}

	changes := Diff(desired, &CurrentState{rawProject: everyFeatureLive()})

	if len(changes) != 20 {
		t.Fatalf("got %d changes, want 20 features", len(changes))
	}
	seen := make(map[string]bool, len(changes))
	for _, change := range changes {
		key, ok := strings.CutPrefix(change.Field, "features.")
		if !ok {
			t.Errorf("change field %q is not under features", change.Field)
			continue
		}
		if seen[key] {
			t.Errorf("feature key %q reported twice", key)
		}
		seen[key] = true
		if got := fmt.Sprint(change.OldValue); got != key {
			t.Errorf("%s old = %q, want %q (wrong live field mapped?)", change.Field, got, key)
		}
		if got := fmt.Sprint(change.NewValue); got != "enabled" {
			t.Errorf("%s new = %q, want enabled", change.Field, got)
		}
	}
}

func TestDiffLeavesUndeclaredFeaturesUnchanged(t *testing.T) {
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec: manifest.RepositorySpec{Features: &manifest.RepositoryFeatures{
			Issues: new(manifest.AccessLevel("enabled")),
		}},
	}
	current := &CurrentState{rawProject: rawProject{
		IssuesAccessLevel: "disabled",
		WikiAccessLevel:   "disabled",
	}}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "features.issues" {
		t.Fatalf("changes = %+v, want only the declared feature to drift", changes)
	}
}

func TestDiffAcceptsPublicAccessLevelFeatures(t *testing.T) {
	desired := &manifest.Repository{
		Metadata: manifest.RepositoryMetadata{Owner: "group", Name: "proj"},
		Spec: manifest.RepositorySpec{Features: &manifest.RepositoryFeatures{
			Pages: new(manifest.PublicAccessLevel("public")),
		}},
	}
	current := &CurrentState{rawProject: rawProject{PagesAccessLevel: "disabled"}}

	changes := Diff(desired, current)

	if len(changes) != 1 || changes[0].Field != "features.pages" {
		t.Fatalf("changes = %+v, want one features.pages update", changes)
	}
	if got := fmt.Sprint(changes[0].NewValue); got != "public" {
		t.Errorf("new value = %q, want public", got)
	}
}
