package manifest

import (
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
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
			DefaultBranch:              new("main"),
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
		"enforce_auth_checks_on_uploads", "archived", "topics", "default_branch",
		"merge_commit_template", "squash_commit_template", "merge_requests_template",
		"features",
		"issues", "repository", "merge_requests", "forking", "ci",
		"container_registry", "analytics", "requirements", "security_and_compliance",
		"wiki", "snippets", "package_registry", "model_experiments", "model_registry",
		"pages", "monitor", "environments", "feature_flags", "infrastructure",
		"releases",
	}
}

// goyaml's literal emitter writes no indentation indicator, so a multiline
// value whose first line starts with whitespace produces unparseable YAML,
// and a newline-only value decodes back as empty. GitLab stores templates
// verbatim (length-validated only), so such values do reach Marshal, which
// must emit a decodable document that reproduces them.
func TestMarshalSurvivesValuesLiteralStyleCannotRepresent(t *testing.T) {
	cases := []struct{ name, value string }{
		{"leading_space_first_line", "  a\nb"},
		{"leading_tab_first_line", "\ta\nb"},
		{"blank_first_line", " \nb"},
		{"newline_only", "\n"},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			doc := fullRepository()
			doc.Spec.MergeCommitTemplate = new(attr.value)

			out, err := Marshal(doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var back Repository
			if err := goyaml.Unmarshal(out, &back); err != nil {
				t.Fatalf("emitted yaml does not parse: %v\n%s", err, out)
			}
			if got := back.Spec.MergeCommitTemplate; got == nil || *got != attr.value {
				t.Errorf("template did not survive the round trip: got %v, want %q\n%s", got, attr.value, out)
			}
		})
	}
}

// A multiline value must be emitted as a literal block (|-), not as a quoted
// string with escape sequences, so templates in the manifest read the way
// they are written in the settings UI.
func TestMarshalEmitsMultilineValuesAsLiteralBlocks(t *testing.T) {
	doc := fullRepository()
	doc.Spec.MergeCommitTemplate = new("%{title}\n\n%{description}")

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "merge_commit_template: |-\n") {
		t.Errorf("multiline template not emitted as a literal block\n%s", out)
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
