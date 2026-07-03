package manifest

import (
	"strings"
	"testing"
)

const levelEnabled = "enabled"

// A document with every optional field set, so marshalling exercises the full
// key sequence.
func fullRepository() *Repository {
	return &Repository{
		APIVersion: APIVersion,
		Kind:       KindRepository,
		Metadata:   RepositoryMetadata{Name: "proj", Owner: "group"},
		Spec: RepositorySpec{
			Description:                new("a tool"),
			Visibility:                 new("private"),
			RequestAccessEnabled:       new(true),
			EnforceAuthChecksOnUploads: new(true),
			Archived:                   new(true),
			Topics:                     []string{"go"},
			MergeCommitTemplate:        new("%{title}"),
			SquashCommitTemplate:       new("%{title}"),
			MergeRequestsTemplate:      new("%{title}"),
			Features: &RepositoryFeatures{
				Issues:                new(levelEnabled),
				Repository:            new(levelEnabled),
				MergeRequests:         new(levelEnabled),
				Forking:               new(levelEnabled),
				CICD:                  new(levelEnabled),
				ContainerRegistry:     new(levelEnabled),
				Analytics:             new(levelEnabled),
				Requirements:          new(levelEnabled),
				SecurityAndCompliance: new(levelEnabled),
				Wiki:                  new(levelEnabled),
				Snippets:              new(levelEnabled),
				PackageRegistry:       new(levelEnabled),
				ModelExperiments:      new(levelEnabled),
				ModelRegistry:         new(levelEnabled),
				Pages:                 new(levelEnabled),
				Monitor:               new(levelEnabled),
				Environments:          new(levelEnabled),
				FeatureFlags:          new(levelEnabled),
				Infrastructure:        new(levelEnabled),
				Releases:              new(levelEnabled),
			},
		},
	}
}

// The spec keys and the features keys in the GitLab settings-UI display order
// the schema promises to emit.
func settingsUIKeyOrder() []string {
	return []string{
		"description", "visibility", "request_access_enabled",
		"enforce_auth_checks_on_uploads", "archived", "topics",
		"merge_commit_template", "squash_commit_template", "merge_requests_template",
		"features",
		"issues", "repository", "merge_requests", "forking", "ci",
		"container_registry", "analytics", "requirements", "security_and_compliance",
		"wiki", "snippets", "package_registry", "model_experiments", "model_registry",
		"pages", "monitor", "environments", "feature_flags", "infrastructure",
		"releases",
	}
}

// The emitted key order is carried solely by the field declaration order of
// the schema structs, so a struct reorder would silently change the output
// without this pin.
func TestRepositoryMarshalsKeysInSettingsUIOrder(t *testing.T) {
	out, err := Marshal(fullRepository())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	section := string(out)
	for _, key := range settingsUIKeyOrder() {
		idx := strings.Index(section, key+":")
		if idx < 0 {
			t.Fatalf("%s missing or out of settings-UI order\n%s", key, out)
		}
		section = section[idx+len(key):]
	}
}
