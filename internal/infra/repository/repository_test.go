package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

const (
	sampleDescription = "a tool"
	visibilityPrivate = "private"
	levelEnabled      = "enabled"
	levelDisabled     = "disabled"
	ownerGroup        = "group"
	nameProj          = "proj"
)

const sampleProjectJSON = `{"description":"a tool","visibility":"private","topics":["go","cli"],"archived":true,` +
	`"issues_access_level":"enabled","merge_requests_access_level":"private","wiki_access_level":"disabled",` +
	`"builds_access_level":"enabled","snippets_access_level":"enabled","container_registry_access_level":"private"}`

var errGlab404 = errors.New("glab api projects/x: exit status 1\n404 Project Not Found")

// wantPtr fails the test unless got points to want.
func wantPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %q", name, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %q, want %q", name, *got, want)
	}
}

func TestFetchRepositoryParsesSettings(t *testing.T) {
	var gotArgs []string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(sampleProjectJSON), nil
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), "group/sub", "proj")
	if err != nil {
		t.Fatalf("FetchRepository: %v", err)
	}
	if state.IsNew {
		t.Errorf("IsNew = true, want false for an existing project")
	}
	if state.Archived == nil || !*state.Archived {
		t.Errorf("archived = false, want true")
	}
	if want := "api projects/group%2Fsub%2Fproj"; strings.Join(gotArgs, " ") != want {
		t.Errorf("glab args = %v, want %q", gotArgs, want)
	}

	for _, check := range []struct{ name, got, want string }{
		{"description", string(state.Description), sampleDescription},
		{"visibility", state.Visibility, visibilityPrivate},
		{"topics", strings.Join(state.Topics, ","), "go,cli"},
		{"issues", state.IssuesAccessLevel, levelEnabled},
		{"wiki", state.WikiAccessLevel, levelDisabled},
		{"builds", state.BuildsAccessLevel, levelEnabled},
		{"container_registry", state.ContainerRegistryAccessLevel, visibilityPrivate},
	} {
		if check.got != check.want {
			t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
		}
	}
}

func TestFetchRepositoryNotFoundIsNew(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errGlab404
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), "group", "missing")
	if err != nil {
		t.Fatalf("FetchRepository should not error on 404, got %v", err)
	}
	if !state.IsNew {
		t.Errorf("IsNew = false, want true for a missing project")
	}
}

func TestToManifest(t *testing.T) {
	state := &CurrentState{
		Owner: ownerGroup,
		Name:  nameProj,
		rawProject: rawProject{
			Description:       sampleDescription,
			Archived:          new(true),
			Visibility:        visibilityPrivate,
			Topics:            []string{"go"},
			IssuesAccessLevel: levelEnabled,
			WikiAccessLevel:   levelDisabled,
		},
	}

	doc := ToManifest(state)

	if doc.APIVersion != manifest.APIVersion {
		t.Errorf("apiVersion = %q, want %q", doc.APIVersion, manifest.APIVersion)
	}
	if doc.Kind != manifest.KindRepository {
		t.Errorf("kind = %q, want %q", doc.Kind, manifest.KindRepository)
	}
	if (doc.Metadata != manifest.RepositoryMetadata{Name: nameProj, Owner: ownerGroup}) {
		t.Errorf("metadata = %+v, want name=proj owner=group", doc.Metadata)
	}
	wantPtr(t, "spec.description", doc.Spec.Description, sampleDescription)
	if doc.Spec.Archived == nil || !*doc.Spec.Archived {
		t.Errorf("spec.archived = %v, want true", doc.Spec.Archived)
	}
	if doc.Spec.Features == nil {
		t.Fatal("spec.features = nil, want populated")
	}
	wantPtr(t, "features.issues", doc.Spec.Features.Issues, levelEnabled)
	wantPtr(t, "features.wiki", doc.Spec.Features.Wiki, levelDisabled)
	// An access level not returned by GitLab is omitted rather than emitted empty.
	if doc.Spec.Features.Snippets != nil {
		t.Errorf("features.snippets = %v, want nil when the access level is absent", doc.Spec.Features.Snippets)
	}
}

func TestToManifestOmitsFeaturesWhenAllEmpty(t *testing.T) {
	doc := ToManifest(&CurrentState{
		Owner:      ownerGroup,
		Name:       nameProj,
		rawProject: rawProject{Visibility: visibilityPrivate},
	})
	if doc.Spec.Features != nil {
		t.Errorf("spec.features = %v, want nil when no access level was reported", doc.Spec.Features)
	}
}

type featureAccessLevel struct {
	apiField string
	yamlKey  string
}

// Every GitLab *_access_level field and its spec.features key, in the
// settings-UI display order the manifest emits. Shared by the mapping and the
// key-order tests so the 20 pairs are declared once.
func featureAccessLevels() []featureAccessLevel {
	return []featureAccessLevel{
		{"issues_access_level", "issues"},
		{"repository_access_level", "repository"},
		{"merge_requests_access_level", "merge_requests"},
		{"forking_access_level", "forking"},
		{"builds_access_level", "ci"},
		{"container_registry_access_level", "container_registry"},
		{"analytics_access_level", "analytics"},
		{"requirements_access_level", "requirements"},
		{"security_and_compliance_access_level", "security_and_compliance"},
		{"wiki_access_level", "wiki"},
		{"snippets_access_level", "snippets"},
		{"package_registry_access_level", "package_registry"},
		{"model_experiments_access_level", "model_experiments"},
		{"model_registry_access_level", "model_registry"},
		{"pages_access_level", "pages"},
		{"monitor_access_level", "monitor"},
		{"environments_access_level", "environments"},
		{"feature_flags_access_level", "feature_flags"},
		{"infrastructure_access_level", "infrastructure"},
		{"releases_access_level", "releases"},
	}
}

// exportYAML runs the import pipeline against a fake glab that returns
// projectJSON, and returns the manifest YAML it produces.
func exportYAML(t *testing.T, projectJSON string) string {
	t.Helper()
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(projectJSON), nil
	})

	state, err := NewClient(runner).FetchRepository(context.Background(), ownerGroup, nameProj)
	if err != nil {
		t.Fatalf("FetchRepository: %v", err)
	}

	out, err := manifest.Marshal(ToManifest(state))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

// Each GitLab *_access_level toggle must round-trip to its own spec.features key.
// One toggle can differ from the naming pattern (ci maps to builds_access_level),
// so each mapping is isolated: only the field under test is present in the API
// response, and the emitted features block must contain exactly that one key.
func TestFetchRepositoryMapsEachFeatureAccessLevel(t *testing.T) {
	for _, feature := range featureAccessLevels() {
		t.Run(feature.yamlKey, func(t *testing.T) {
			out := exportYAML(t, fmt.Sprintf(`{"visibility":"private","%s":"enabled"}`, feature.apiField))

			var doc struct {
				Spec struct {
					Features map[string]string `yaml:"features"`
				} `yaml:"spec"`
			}
			if err := goyaml.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, out)
			}
			if got := len(doc.Spec.Features); got != 1 {
				t.Fatalf("spec.features has %d keys, want exactly 1 (%q)\n%s", got, feature.yamlKey, out)
			}
			if got := doc.Spec.Features[feature.yamlKey]; got != levelEnabled {
				t.Errorf("spec.features.%s = %q, want %q\n%s", feature.yamlKey, got, levelEnabled, out)
			}
		})
	}
}

// pages_access_level and package_registry_access_level are the two toggles that
// also accept "public" (the UI labels the latter "Allow anyone to pull from
// Package Registry"). Access levels are stored and emitted as raw strings, so
// this value must survive untouched rather than being restricted to the
// three-value set the others use.
func TestToManifestPreservesPublicAccessLevel(t *testing.T) {
	doc := ToManifest(&CurrentState{
		Owner: ownerGroup,
		Name:  nameProj,
		rawProject: rawProject{
			PagesAccessLevel:           "public",
			PackageRegistryAccessLevel: "public",
		},
	})
	if doc.Spec.Features == nil {
		t.Fatal("spec.features = nil, want populated")
	}
	wantPtr(t, "features.pages", doc.Spec.Features.Pages, "public")
	wantPtr(t, "features.package_registry", doc.Spec.Features.PackageRegistry, "public")
}

// The two cases mirror true/false so a swapped mapping between the fields
// cannot pass, and false must survive export as an intentional setting.
func TestFetchRepositoryMapsVisibilityBooleans(t *testing.T) {
	cases := []struct {
		name        string
		projectJSON string
		want        []string
	}{
		{
			name:        "request_access_enabled_true",
			projectJSON: `{"visibility":"private","request_access_enabled":true,"enforce_auth_checks_on_uploads":false}`,
			want:        []string{"request_access_enabled: true", "enforce_auth_checks_on_uploads: false"},
		},
		{
			name:        "enforce_auth_checks_on_uploads_true",
			projectJSON: `{"visibility":"private","request_access_enabled":false,"enforce_auth_checks_on_uploads":true}`,
			want:        []string{"request_access_enabled: false", "enforce_auth_checks_on_uploads: true"},
		},
	}

	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			out := exportYAML(t, attr.projectJSON)
			for _, want := range attr.want {
				if !strings.Contains(out, want) {
					t.Errorf("yaml missing %q\n%s", want, out)
				}
			}
		})
	}
}

// exportSpec runs the import pipeline against a fake glab that returns
// projectJSON, and returns the emitted YAML together with its decoded spec
// block.
func exportSpec(t *testing.T, projectJSON string) (string, map[string]any) {
	t.Helper()
	out := exportYAML(t, projectJSON)

	var doc struct {
		Spec map[string]any `yaml:"spec"`
	}
	if err := goyaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	return out, doc.Spec
}

// Each commit/description template must round-trip to its own spec key: one
// template per case, so a swapped mapping between the three cannot pass. The
// non-selected templates are explicit null — GitLab's wire format for an
// unset template — which must be omitted rather than exported as empty.
func TestFetchRepositoryMapsMergeTemplates(t *testing.T) {
	templates := []string{"merge_commit_template", "squash_commit_template", "merge_requests_template"}
	for _, field := range templates {
		t.Run(field, func(t *testing.T) {
			attrs := make([]string, 0, 1+len(templates))
			attrs = append(attrs, `"visibility":"private"`)
			for _, key := range templates {
				value := "null"
				if key == field {
					value = `"%{title}"`
				}
				attrs = append(attrs, fmt.Sprintf("%q:%s", key, value))
			}
			out, spec := exportSpec(t, "{"+strings.Join(attrs, ",")+"}")
			if got := spec[field]; got != "%{title}" {
				t.Errorf("spec.%s = %v, want %q\n%s", field, got, "%{title}", out)
			}
			for _, other := range templates {
				if _, ok := spec[other]; other != field && ok {
					t.Errorf("spec.%s present, want only %s set\n%s", other, field, out)
				}
			}
		})
	}
}

// Templates hold multiline text, which must survive the YAML round trip
// byte for byte.
func TestFetchRepositoryPreservesMultilineTemplate(t *testing.T) {
	out := exportYAML(t, `{"visibility":"private","merge_commit_template":"%{title}\n\n%{description}"}`)

	var doc struct {
		Spec struct {
			MergeCommitTemplate string `yaml:"merge_commit_template"`
		} `yaml:"spec"`
	}
	if err := goyaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if want := "%{title}\n\n%{description}"; doc.Spec.MergeCommitTemplate != want {
		t.Errorf("spec.merge_commit_template = %q, want %q\n%s", doc.Spec.MergeCommitTemplate, want, out)
	}
}

// Free-text fields edited in the GitLab web UI are stored with CRLF line
// endings, and goyaml writes such values out with CR in the YAML byte stream
// itself, so the emitted document gets mixed line endings. A bare CR counts
// too: goyaml treats it as multiline content and would emit it into a literal
// block. The raw output is asserted because reparsing normalizes line breaks
// and would hide the CR. One field per case so a field skipped by the
// normalization cannot pass.
func TestFetchRepositoryNormalizesCRLFToLF(t *testing.T) {
	fields := []string{"description", "merge_commit_template", "squash_commit_template", "merge_requests_template"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			out, spec := exportSpec(t, fmt.Sprintf(`{"visibility":"private","%s":"a\r\nb\rc"}`, field))

			if strings.Contains(out, "\r") || strings.Contains(out, `\r`) {
				t.Errorf("yaml carries CR for %s\n%q", field, out)
			}
			if got := spec[field]; got != "a\nb\nc" {
				t.Errorf("spec.%s = %q, want %q\n%s", field, got, "a\nb\nc", out)
			}
		})
	}
}

// A boolean attribute the API did not return must be omitted, not emitted as
// false. archived is included so all spec booleans share one absence rule.
func TestFetchRepositoryOmitsBooleansWhenAbsent(t *testing.T) {
	out := exportYAML(t, `{"visibility":"private"}`)
	for _, key := range []string{"request_access_enabled", "enforce_auth_checks_on_uploads", "archived"} {
		if strings.Contains(out, key) {
			t.Errorf("yaml contains %q, want it omitted when the API did not report it\n%s", key, out)
		}
	}
}
