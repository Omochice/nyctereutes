// Package repository fetches a GitLab project's current basic settings and
// converts them into a manifest document for the infra import command.
package repository

import (
	"context"
	"encoding/json"
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
		if isNotFound(err) {
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
	return state, nil
}

// The subset of the `glab api projects/:id` JSON response the import reads.
type rawProject struct {
	Description string   `json:"description"`
	Visibility  string   `json:"visibility"`
	Topics      []string `json:"topics"`
	// Pointer booleans and templates keep "not reported" (JSON absence or
	// null) apart from an intentional false or empty string.
	Archived                   *bool   `json:"archived"`
	RequestAccessEnabled       *bool   `json:"request_access_enabled"`
	EnforceAuthChecksOnUploads *bool   `json:"enforce_auth_checks_on_uploads"`
	MergeCommitTemplate        *string `json:"merge_commit_template"`
	SquashCommitTemplate       *string `json:"squash_commit_template"`
	MergeRequestsTemplate      *string `json:"merge_requests_template"`
	// Per-feature access levels, in GitLab settings-UI display order; empty
	// when GitLab did not report the field.
	IssuesAccessLevel                string `json:"issues_access_level"`
	RepositoryAccessLevel            string `json:"repository_access_level"`
	MergeRequestsAccessLevel         string `json:"merge_requests_access_level"`
	ForkingAccessLevel               string `json:"forking_access_level"`
	BuildsAccessLevel                string `json:"builds_access_level"`
	ContainerRegistryAccessLevel     string `json:"container_registry_access_level"`
	AnalyticsAccessLevel             string `json:"analytics_access_level"`
	RequirementsAccessLevel          string `json:"requirements_access_level"`
	SecurityAndComplianceAccessLevel string `json:"security_and_compliance_access_level"`
	WikiAccessLevel                  string `json:"wiki_access_level"`
	SnippetsAccessLevel              string `json:"snippets_access_level"`
	PackageRegistryAccessLevel       string `json:"package_registry_access_level"`
	ModelExperimentsAccessLevel      string `json:"model_experiments_access_level"`
	ModelRegistryAccessLevel         string `json:"model_registry_access_level"`
	PagesAccessLevel                 string `json:"pages_access_level"`
	MonitorAccessLevel               string `json:"monitor_access_level"`
	EnvironmentsAccessLevel          string `json:"environments_access_level"`
	FeatureFlagsAccessLevel          string `json:"feature_flags_access_level"`
	InfrastructureAccessLevel        string `json:"infrastructure_access_level"`
	ReleasesAccessLevel              string `json:"releases_access_level"`
}

// Unmarshals a `glab api projects/:id` response into a CurrentState. Owner and
// Name are not carried by the response and are set by the caller.
func parseProject(out []byte) (*CurrentState, error) {
	var raw rawProject
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal project json: %w", err)
	}

	raw.normalizeNewlines()
	return &CurrentState{rawProject: raw}, nil
}

// The web UI stores CRLF in free-text fields, and goyaml would carry the CR
// into the emitted document as mixed line endings, so line breaks are
// normalized to LF at the wire boundary.
func (raw *rawProject) normalizeNewlines() {
	raw.Description = strings.ReplaceAll(raw.Description, "\r\n", "\n")
	for _, template := range []*string{raw.MergeCommitTemplate, raw.SquashCommitTemplate, raw.MergeRequestsTemplate} {
		if template != nil {
			*template = strings.ReplaceAll(*template, "\r\n", "\n")
		}
	}
}

// Reports whether err is a GitLab 404. It matches the status in the glab error
// text, mirroring how the dep client detects an already-approved 401.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "404")
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
			Description:                new(state.Description),
			Visibility:                 new(state.Visibility),
			RequestAccessEnabled:       state.RequestAccessEnabled,
			EnforceAuthChecksOnUploads: state.EnforceAuthChecksOnUploads,
			Archived:                   state.Archived,
			Topics:                     state.Topics,
			MergeCommitTemplate:        state.MergeCommitTemplate,
			SquashCommitTemplate:       state.SquashCommitTemplate,
			MergeRequestsTemplate:      state.MergeRequestsTemplate,
			Features:                   toFeatures(state),
		},
	}
}

// Builds the features block, or nil when no access level was reported so the
// whole block is omitted rather than emitted empty.
func toFeatures(state *CurrentState) *manifest.RepositoryFeatures {
	features := manifest.RepositoryFeatures{
		Issues:                accessLevel(state.IssuesAccessLevel),
		Repository:            accessLevel(state.RepositoryAccessLevel),
		MergeRequests:         accessLevel(state.MergeRequestsAccessLevel),
		Forking:               accessLevel(state.ForkingAccessLevel),
		CICD:                  accessLevel(state.BuildsAccessLevel),
		ContainerRegistry:     accessLevel(state.ContainerRegistryAccessLevel),
		Analytics:             accessLevel(state.AnalyticsAccessLevel),
		Requirements:          accessLevel(state.RequirementsAccessLevel),
		SecurityAndCompliance: accessLevel(state.SecurityAndComplianceAccessLevel),
		Wiki:                  accessLevel(state.WikiAccessLevel),
		Snippets:              accessLevel(state.SnippetsAccessLevel),
		PackageRegistry:       accessLevel(state.PackageRegistryAccessLevel),
		ModelExperiments:      accessLevel(state.ModelExperimentsAccessLevel),
		ModelRegistry:         accessLevel(state.ModelRegistryAccessLevel),
		Pages:                 accessLevel(state.PagesAccessLevel),
		Monitor:               accessLevel(state.MonitorAccessLevel),
		Environments:          accessLevel(state.EnvironmentsAccessLevel),
		FeatureFlags:          accessLevel(state.FeatureFlagsAccessLevel),
		Infrastructure:        accessLevel(state.InfrastructureAccessLevel),
		Releases:              accessLevel(state.ReleasesAccessLevel),
	}
	if features == (manifest.RepositoryFeatures{}) {
		return nil
	}
	return &features
}

// nil for an unreported level, so omitempty drops the field.
func accessLevel(level string) *string {
	if level == "" {
		return nil
	}
	return new(level)
}
