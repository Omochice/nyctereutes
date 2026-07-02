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
	if !state.Archived {
		t.Errorf("archived = false, want true")
	}
	if want := "api projects/group%2Fsub%2Fproj"; strings.Join(gotArgs, " ") != want {
		t.Errorf("glab args = %v, want %q", gotArgs, want)
	}

	for _, check := range []struct{ name, got, want string }{
		{"description", state.Description, sampleDescription},
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
		Owner:             ownerGroup,
		Name:              nameProj,
		Description:       sampleDescription,
		Archived:          true,
		Visibility:        visibilityPrivate,
		Topics:            []string{"go"},
		IssuesAccessLevel: levelEnabled,
		WikiAccessLevel:   levelDisabled,
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
	doc := ToManifest(&CurrentState{Owner: ownerGroup, Name: nameProj, Visibility: visibilityPrivate})
	if doc.Spec.Features != nil {
		t.Errorf("spec.features = %v, want nil when no access level was reported", doc.Spec.Features)
	}
}

// Each GitLab *_access_level toggle must round-trip to its own spec.features key.
// One toggle can differ from the naming pattern (ci maps to builds_access_level),
// so each mapping is isolated: only the field under test is present in the API
// response, and the emitted features block must contain exactly that one key.
func TestFetchRepositoryMapsEachFeatureAccessLevel(t *testing.T) {
	cases := []struct {
		apiField string
		yamlKey  string
	}{
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

	for _, feature := range cases {
		t.Run(feature.yamlKey, func(t *testing.T) {
			projectJSON := fmt.Sprintf(`{"visibility":"private","%s":"enabled"}`, feature.apiField)
			runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
				return []byte(projectJSON), nil
			})

			state, err := NewClient(runner).FetchRepository(context.Background(), ownerGroup, nameProj)
			if err != nil {
				t.Fatalf("FetchRepository: %v", err)
			}

			out, err := goyaml.Marshal(ToManifest(state))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var doc struct {
				Spec struct {
					Features map[string]string `yaml:"features"`
				} `yaml:"spec"`
			}
			if err := goyaml.Unmarshal(out, &doc); err != nil {
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
		Owner:                      ownerGroup,
		Name:                       nameProj,
		PagesAccessLevel:           "public",
		PackageRegistryAccessLevel: "public",
	})
	if doc.Spec.Features == nil {
		t.Fatal("spec.features = nil, want populated")
	}
	wantPtr(t, "features.pages", doc.Spec.Features.Pages, "public")
	wantPtr(t, "features.package_registry", doc.Spec.Features.PackageRegistry, "public")
}

// spec.features keys must be emitted in the GitLab settings-UI display order.
// The order is carried only by the field declaration order of
// manifest.RepositoryFeatures, so a struct reorder would silently change the
// output without this pin.
func TestToManifestEmitsFeaturesInSettingsUIOrder(t *testing.T) {
	state := &CurrentState{
		Owner:                            ownerGroup,
		Name:                             nameProj,
		IssuesAccessLevel:                levelEnabled,
		RepositoryAccessLevel:            levelEnabled,
		MergeRequestsAccessLevel:         levelEnabled,
		ForkingAccessLevel:               levelEnabled,
		BuildsAccessLevel:                levelEnabled,
		ContainerRegistryAccessLevel:     levelEnabled,
		AnalyticsAccessLevel:             levelEnabled,
		RequirementsAccessLevel:          levelEnabled,
		SecurityAndComplianceAccessLevel: levelEnabled,
		WikiAccessLevel:                  levelEnabled,
		SnippetsAccessLevel:              levelEnabled,
		PackageRegistryAccessLevel:       levelEnabled,
		ModelExperimentsAccessLevel:      levelEnabled,
		ModelRegistryAccessLevel:         levelEnabled,
		PagesAccessLevel:                 levelEnabled,
		MonitorAccessLevel:               levelEnabled,
		EnvironmentsAccessLevel:          levelEnabled,
		FeatureFlagsAccessLevel:          levelEnabled,
		InfrastructureAccessLevel:        levelEnabled,
		ReleasesAccessLevel:              levelEnabled,
	}

	out, err := goyaml.Marshal(ToManifest(state))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	wantOrder := []string{
		"issues", "repository", "merge_requests", "forking", "ci",
		"container_registry", "analytics", "requirements", "security_and_compliance",
		"wiki", "snippets", "package_registry", "model_experiments", "model_registry",
		"pages", "monitor", "environments", "feature_flags", "infrastructure", "releases",
	}
	yamlText := string(out)
	start := strings.Index(yamlText, "features:")
	if start < 0 {
		t.Fatalf("features block missing\n%s", out)
	}
	section := yamlText[start:]
	for _, key := range wantOrder {
		idx := strings.Index(section, key+":")
		if idx < 0 {
			t.Fatalf("features.%s missing or out of settings-UI order\n%s", key, out)
		}
		section = section[idx+len(key):]
	}
}
