// Package repository fetches a GitLab project's current basic settings and
// converts them into a manifest document for the infra import command.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The current basic settings of a GitLab project. The embedded rawProject
// carries every attribute read from the API, so each attribute is declared
// once; only the fields GitLab does not report live here.
type CurrentState struct {
	rawProject

	Owner string
	Name  string
	IsNew bool // true when the project does not exist on GitLab
	// GitLab's isCatalogResource, read over GraphQL because the projects REST
	// endpoint does not carry the catalog status.
	CatalogResource bool
}

// Drives the glab CLI to read GitLab project state.
type Client struct {
	runner glab.Runner
}

// Builds a Client that runs glab through runner.
func NewClient(runner glab.Runner) *Client {
	return &Client{runner: runner}
}

// Fetches one GitLab project's basic settings. A missing project (404) is not an
// error: it yields a CurrentState with IsNew set, so the caller can report it.
func (c *Client) FetchRepository(ctx context.Context, owner, name string) (*CurrentState, error) {
	out, err := c.runner.Run(ctx, "api", "projects/"+glab.EncodePath(owner+"/"+name))
	if err != nil {
		// A classified 404 means the project is absent, not a real failure; the
		// sentinel, not the error text, keeps an unrelated failure that merely
		// mentions 404 from being mistaken for a missing project.
		if errors.Is(err, glab.ErrNotFound) {
			return &CurrentState{Owner: owner, Name: name, IsNew: true}, nil
		}
		return nil, fmt.Errorf("fetch project %s/%s: %w", owner, name, err)
	}

	state, err := parseProject(out)
	if err != nil {
		return nil, fmt.Errorf("parse project %s/%s: %w", owner, name, err)
	}
	state.Owner = owner
	state.Name = name

	// The catalog status lives only in GraphQL, so it is a second call rather
	// than a field on the REST response parsed above.
	catalog, err := c.fetchCatalogResource(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	state.CatalogResource = catalog
	return state, nil
}

// The GraphQL query reading a project's CI/CD Catalog status. isCatalogResource
// is the only field the projects REST endpoint omits, so it is fetched on its
// own. fullPath is the namespace-qualified path GraphQL addresses a project by,
// which unlike the REST endpoint is not percent-encoded.
const catalogResourceQuery = `query($fullPath: ID!) { project(fullPath: $fullPath) { isCatalogResource } }`

// Signals a GraphQL query that resolved to a null project. glab reports a
// top-level GraphQL error through a non-zero exit, so the read below only
// reaches a null project on an otherwise successful response.
var errCatalogProjectMissing = errors.New("project not visible over GraphQL")

// Reads whether a project is published to the CI/CD Catalog. A null
// isCatalogResource (the field is Experiment and may be withheld) counts as
// not-a-resource so a project that cannot report it is simply left unmanaged.
func (c *Client) fetchCatalogResource(ctx context.Context, owner, name string) (bool, error) {
	out, err := c.runner.Run(ctx, "api", "graphql",
		"-f", "query="+catalogResourceQuery,
		"-f", "fullPath="+owner+"/"+name)
	if err != nil {
		return false, fmt.Errorf("fetch catalog status %s/%s: %w", owner, name, err)
	}

	var resp struct {
		Data struct {
			Project *struct {
				IsCatalogResource *bool `json:"isCatalogResource"`
			} `json:"project"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return false, fmt.Errorf("parse catalog status %s/%s: %w", owner, name, err)
	}
	// GraphQL returns a null project when it cannot see one the REST fetch just
	// resolved; treating that as not-a-resource would silently emit a wrong
	// value, so it is surfaced rather than swallowed.
	if resp.Data.Project == nil {
		return false, fmt.Errorf("fetch catalog status %s/%s: %w", owner, name, errCatalogProjectMissing)
	}
	return resp.Data.Project.IsCatalogResource != nil && *resp.Data.Project.IsCatalogResource, nil
}

// Free text that GitLab stores verbatim (length-validated only). The web UI
// saves CRLF and a YAML literal block cannot carry a bare CR, so line endings
// are normalized to LF while decoding; a field gets this by declaring the
// type.
type freeText string

// Normalizes line endings to LF while decoding.
func (text *freeText) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("unmarshal free text: %w", err)
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	*text = freeText(strings.ReplaceAll(value, "\r", "\n"))
	return nil
}

// Text where the empty string never occurs as a real value, so "" doubles as
// "not reported by GitLab". The distinct type makes ToManifest's conversion
// choice a compile-time concern: such fields only fit optional, which drops
// the key, while new(state.X) would produce an incompatible pointer.
type reported string

// The subset of the `glab api projects/:id` JSON response the import reads.
type rawProject struct {
	Description freeText `json:"description"`
	Visibility  reported `json:"visibility"`
	Topics      []string `json:"topics"`
	// Empty when the repository has no commits yet: GitLab reports null.
	DefaultBranch reported `json:"default_branch"`
	// GitLab always reports a merge_method: "merge", "rebase_merge" or "ff".
	MergeMethod reported `json:"merge_method"`
	// Pointer booleans and templates keep "not reported" (JSON absence or
	// null) apart from an intentional false or empty string.
	Archived                   *bool `json:"archived"`
	RequestAccessEnabled       *bool `json:"request_access_enabled"`
	EnforceAuthChecksOnUploads *bool `json:"enforce_auth_checks_on_uploads"`
	// Merge checks; the gates on a green pipeline and on resolved threads.
	OnlyAllowMergeIfPipelineSucceeds          *bool     `json:"only_allow_merge_if_pipeline_succeeds"`
	AllowMergeOnSkippedPipeline               *bool     `json:"allow_merge_on_skipped_pipeline"`
	OnlyAllowMergeIfAllDiscussionsAreResolved *bool     `json:"only_allow_merge_if_all_discussions_are_resolved"`
	MergeCommitTemplate                       *freeText `json:"merge_commit_template"`
	SquashCommitTemplate                      *freeText `json:"squash_commit_template"`
	MergeRequestsTemplate                     *freeText `json:"merge_requests_template"`
	// Per-feature access levels, in GitLab settings-UI display order.
	IssuesAccessLevel                reported `json:"issues_access_level"`
	RepositoryAccessLevel            reported `json:"repository_access_level"`
	MergeRequestsAccessLevel         reported `json:"merge_requests_access_level"`
	ForkingAccessLevel               reported `json:"forking_access_level"`
	BuildsAccessLevel                reported `json:"builds_access_level"`
	ContainerRegistryAccessLevel     reported `json:"container_registry_access_level"`
	AnalyticsAccessLevel             reported `json:"analytics_access_level"`
	RequirementsAccessLevel          reported `json:"requirements_access_level"`
	SecurityAndComplianceAccessLevel reported `json:"security_and_compliance_access_level"`
	WikiAccessLevel                  reported `json:"wiki_access_level"`
	SnippetsAccessLevel              reported `json:"snippets_access_level"`
	PackageRegistryAccessLevel       reported `json:"package_registry_access_level"`
	ModelExperimentsAccessLevel      reported `json:"model_experiments_access_level"`
	ModelRegistryAccessLevel         reported `json:"model_registry_access_level"`
	PagesAccessLevel                 reported `json:"pages_access_level"`
	MonitorAccessLevel               reported `json:"monitor_access_level"`
	EnvironmentsAccessLevel          reported `json:"environments_access_level"`
	FeatureFlagsAccessLevel          reported `json:"feature_flags_access_level"`
	InfrastructureAccessLevel        reported `json:"infrastructure_access_level"`
	ReleasesAccessLevel              reported `json:"releases_access_level"`
}

// Unmarshals a `glab api projects/:id` response into a CurrentState. Owner and
// Name are not carried by the response and are set by the caller.
func parseProject(out []byte) (*CurrentState, error) {
	var raw rawProject
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal project json: %w", err)
	}

	return &CurrentState{rawProject: raw}, nil
}

// Converts current state into a Repository manifest document, emitting only the
// GitLab basic settings.
func ToManifest(state *CurrentState) *manifest.Repository {
	return &manifest.Repository{
		APIVersion: manifest.APIVersion,
		Kind:       manifest.KindRepository,
		Metadata: manifest.RepositoryMetadata{
			Name:  state.Name,
			Owner: state.Owner,
		},
		Spec: manifest.RepositorySpec{
			Description:                      new(string(state.Description)),
			Visibility:                       optional[manifest.Visibility](state.Visibility),
			RequestAccessEnabled:             state.RequestAccessEnabled,
			EnforceAuthChecksOnUploads:       state.EnforceAuthChecksOnUploads,
			Archived:                         state.Archived,
			CICatalog:                        new(state.CatalogResource),
			Topics:                           state.Topics,
			DefaultBranch:                    optional[string](state.DefaultBranch),
			MergeMethod:                      optional[manifest.MergeMethod](state.MergeMethod),
			OnlyAllowMergeIfPipelineSucceeds: state.OnlyAllowMergeIfPipelineSucceeds,
			AllowMergeOnSkippedPipeline:      state.AllowMergeOnSkippedPipeline,
			OnlyAllowMergeIfAllDiscussionsAreResolved: state.OnlyAllowMergeIfAllDiscussionsAreResolved,
			MergeCommitTemplate:                       (*string)(state.MergeCommitTemplate),
			SquashCommitTemplate:                      (*string)(state.SquashCommitTemplate),
			MergeRequestsTemplate:                     (*string)(state.MergeRequestsTemplate),
			Features:                                  toFeatures(state),
		},
	}
}

// Builds the features block, or nil when no access level was reported so the
// whole block is omitted rather than emitted empty.
func toFeatures(state *CurrentState) *manifest.RepositoryFeatures {
	features := manifest.RepositoryFeatures{
		Issues:                optional[manifest.AccessLevel](state.IssuesAccessLevel),
		Repository:            optional[manifest.AccessLevel](state.RepositoryAccessLevel),
		MergeRequests:         optional[manifest.AccessLevel](state.MergeRequestsAccessLevel),
		Forking:               optional[manifest.AccessLevel](state.ForkingAccessLevel),
		CICD:                  optional[manifest.AccessLevel](state.BuildsAccessLevel),
		ContainerRegistry:     optional[manifest.AccessLevel](state.ContainerRegistryAccessLevel),
		Analytics:             optional[manifest.AccessLevel](state.AnalyticsAccessLevel),
		Requirements:          optional[manifest.AccessLevel](state.RequirementsAccessLevel),
		SecurityAndCompliance: optional[manifest.AccessLevel](state.SecurityAndComplianceAccessLevel),
		Wiki:                  optional[manifest.AccessLevel](state.WikiAccessLevel),
		Snippets:              optional[manifest.AccessLevel](state.SnippetsAccessLevel),
		PackageRegistry:       optional[manifest.PublicAccessLevel](state.PackageRegistryAccessLevel),
		ModelExperiments:      optional[manifest.AccessLevel](state.ModelExperimentsAccessLevel),
		ModelRegistry:         optional[manifest.AccessLevel](state.ModelRegistryAccessLevel),
		Pages:                 optional[manifest.PublicAccessLevel](state.PagesAccessLevel),
		Monitor:               optional[manifest.AccessLevel](state.MonitorAccessLevel),
		Environments:          optional[manifest.AccessLevel](state.EnvironmentsAccessLevel),
		FeatureFlags:          optional[manifest.AccessLevel](state.FeatureFlagsAccessLevel),
		Infrastructure:        optional[manifest.AccessLevel](state.InfrastructureAccessLevel),
		Releases:              optional[manifest.AccessLevel](state.ReleasesAccessLevel),
	}
	if features == (manifest.RepositoryFeatures{}) {
		return nil
	}
	return &features
}

// nil for a value GitLab did not report, so omitempty drops the field. The
// type parameter picks the manifest value type the target field carries.
func optional[Value ~string](value reported) *Value {
	if value == "" {
		return nil
	}
	return new(Value(value))
}
