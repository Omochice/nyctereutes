package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/color"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var (
	errPlanNeedsPath = errors.New("plan requires at least one <path>")
	errPlanDrift     = errors.New("changes detected")
	errPlanFailed    = errors.New("plan failed")
)

type planCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner

	CI bool `long:"ci" description:"exit non-zero when any drift is detected"`
}

// Shows how each declared manifest differs from its live GitLab project.
func (c *planCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errPlanNeedsPath
	}

	ctx := context.Background()
	client := repository.NewClient(c.runner)
	colorize := color.Enabled(c.inout.Stdout)
	changed := 0
	failures := 0
	for _, path := range args {
		files, err := manifestFiles(path)
		if err != nil {
			_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", err)
			failures++
			continue
		}
		for _, file := range files {
			fileChanged, fileFailures := c.planFile(ctx, client, file, colorize)
			changed += fileChanged
			failures += fileFailures
		}
	}

	if changed == 0 && failures == 0 {
		_, _ = fmt.Fprintln(c.inout.Stdout, "No changes.")
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d problem(s)", errPlanFailed, failures)
	}
	// Drift is reported, not an error, so a human run always succeeds; --ci
	// turns detected drift into a non-zero exit for pipeline gating.
	if c.CI && changed > 0 {
		return errPlanDrift
	}
	return nil
}

// Plans each document in one file against its live project. Problems are
// reported and counted rather than fatal, so one bad document or project never
// hides the rest.
func (c *planCommand) planFile(
	ctx context.Context, client *repository.Client, file string, colorize bool,
) (changed, failures int) {
	repos, failures := readManifestFile(c.inout.Stderr, file)
	for _, repo := range repos {
		state, failed := fetchState(ctx, client, c.inout.Stderr, repo)
		failures += failed
		if state == nil {
			continue
		}
		changes := repository.Diff(repo, state)
		if len(changes) == 0 {
			continue
		}
		changed++
		printChanges(c.inout.Stdout, repo.Metadata.Owner+"/"+repo.Metadata.Name, changes, colorize)
	}
	return changed, failures
}
