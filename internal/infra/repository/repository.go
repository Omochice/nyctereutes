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

// The current basic settings of a GitLab project.
type CurrentState struct {
	Owner       string
	Name        string
	IsNew       bool // true when the project does not exist on GitLab
	Description string
	Archived    bool
	Visibility  string
	Topics      []string
	// Per-feature access levels; empty when GitLab did not report the field.
	IssuesAccessLevel            string
	MergeRequestsAccessLevel     string
	WikiAccessLevel              string
	BuildsAccessLevel            string
	SnippetsAccessLevel          string
	ContainerRegistryAccessLevel string
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

	var raw struct {
		Description                  string   `json:"description"`
		Visibility                   string   `json:"visibility"`
		Topics                       []string `json:"topics"`
		Archived                     bool     `json:"archived"`
		IssuesAccessLevel            string   `json:"issues_access_level"`
		MergeRequestsAccessLevel     string   `json:"merge_requests_access_level"`
		WikiAccessLevel              string   `json:"wiki_access_level"`
		BuildsAccessLevel            string   `json:"builds_access_level"`
		SnippetsAccessLevel          string   `json:"snippets_access_level"`
		ContainerRegistryAccessLevel string   `json:"container_registry_access_level"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse project %s/%s: %w", owner, name, err)
	}

	return &CurrentState{
		Owner:                        owner,
		Name:                         name,
		Description:                  raw.Description,
		Archived:                     raw.Archived,
		Visibility:                   raw.Visibility,
		Topics:                       raw.Topics,
		IssuesAccessLevel:            raw.IssuesAccessLevel,
		MergeRequestsAccessLevel:     raw.MergeRequestsAccessLevel,
		WikiAccessLevel:              raw.WikiAccessLevel,
		BuildsAccessLevel:            raw.BuildsAccessLevel,
		SnippetsAccessLevel:          raw.SnippetsAccessLevel,
		ContainerRegistryAccessLevel: raw.ContainerRegistryAccessLevel,
	}, nil
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
			Description: new(state.Description),
			Visibility:  new(state.Visibility),
			Archived:    new(state.Archived),
			Topics:      state.Topics,
			Features:    toFeatures(state),
		},
	}
}

// Builds the features block, or nil when no access level was reported so the
// whole block is omitted rather than emitted empty.
func toFeatures(state *CurrentState) *manifest.RepositoryFeatures {
	features := manifest.RepositoryFeatures{
		Issues:            accessLevel(state.IssuesAccessLevel),
		MergeRequests:     accessLevel(state.MergeRequestsAccessLevel),
		Wiki:              accessLevel(state.WikiAccessLevel),
		CICD:              accessLevel(state.BuildsAccessLevel),
		Snippets:          accessLevel(state.SnippetsAccessLevel),
		ContainerRegistry: accessLevel(state.ContainerRegistryAccessLevel),
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
