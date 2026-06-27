package nyctereutes

import (
	"context"
	"errors"
	"fmt"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/config"
	"github.com/Omochice/nyctereutes/internal/gitlab"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/types"
	"github.com/Omochice/nyctereutes/internal/ui"
)

var (
	errInvalidMergeMethod = errors.New("invalid merge method")
	errGroupNotFound      = errors.New("group not found")
)

// scopeFlags are the search-scope flags shared by list, approve and merge.
// Repo and Author are pointers so an explicit (even empty) flag can be told
// apart from "not specified", in which case config or a default is used.
type scopeFlags struct {
	Repo      *string `short:"R" long:"repo" description:"Target project(s) (GROUP/PROJECT), comma-separated"`
	Author    *string `long:"author" description:"MR author username (defaults to the Renovate bot)"`
	Label     string  `long:"label" description:"MR label to filter"`
	GroupPath *string `long:"group-path" description:"Target GitLab group/subgroup full path"`
	Reviewer  string  `long:"reviewer" description:"Filter MRs by reviewer username"`
	Limit     int     `long:"limit" default:"200" description:"Max MRs to fetch per author across the scope"`
}

func (s scopeFlags) resolve(ctx context.Context, runner glab.Runner) (gitlab.SearchParams, []string) {
	cfg := config.Load(ctx, runner)
	group, repos := gitlab.ResolveScope(s.Repo, s.GroupPath, cfg.Repos)
	authors := gitlab.ResolveAuthors(s.Author, cfg.Author)
	return gitlab.SearchParams{
		Group:    group,
		Repos:    repos,
		Label:    s.Label,
		Authors:  authors,
		Limit:    s.Limit,
		Reviewer: s.Reviewer,
	}, cfg.Patterns
}

type depCommand struct {
	inout *cli.ProcInout

	List    *depListCommand    `command:"list" description:"list dependency MRs"`
	Approve *depApproveCommand `command:"approve" description:"bulk approve a group of MRs"`
	Merge   *depMergeCommand   `command:"merge" description:"bulk merge a group of MRs"`
}

func newDepCommand(inout *cli.ProcInout, runner glab.Runner) *depCommand {
	return &depCommand{
		inout:   inout,
		List:    &depListCommand{inout: inout, runner: runner},
		Approve: &depApproveCommand{inout: inout, runner: runner},
		Merge:   &depMergeCommand{inout: inout, runner: runner},
	}
}

// Execute runs when "dep" is invoked with no subcommand. It is reserved for a
// future TUI; for now it reports that it is not implemented.
func (c *depCommand) Execute(_ []string) error {
	_, _ = fmt.Fprintln(c.inout.Stderr, "not implemented")
	return errNotImplemented
}

type depListCommand struct {
	scopeFlags

	inout  *cli.ProcInout
	runner glab.Runner

	Group bool `long:"group" description:"Group MRs by package@version"`
	JSON  bool `long:"json" description:"Output as JSON"`
}

func (c *depListCommand) Execute(_ []string) error {
	ctx := context.Background()
	params, patterns := c.resolve(ctx, c.runner)

	mrs, err := gitlab.NewClient(c.runner).SearchMRs(ctx, params)
	if err != nil {
		return fmt.Errorf("search MRs: %w", err)
	}
	if len(mrs) == 0 {
		_, _ = fmt.Fprintln(c.inout.Stdout, "No dependency MRs found")
		return nil
	}

	if c.Group {
		groups := gitlab.GroupMRs(mrs, patterns)
		if err := ui.NewFromGroups(c.inout.Stdout, groups, c.JSON).DisplayGroups(groups); err != nil {
			return fmt.Errorf("display groups: %w", err)
		}
		return nil
	}
	if err := ui.New(c.inout.Stdout, mrs, c.JSON).DisplayList(mrs); err != nil {
		return fmt.Errorf("display list: %w", err)
	}
	return nil
}

type depApproveCommand struct {
	scopeFlags

	inout  *cli.ProcInout
	runner glab.Runner

	Group  string `long:"group" required:"true" description:"Group key (package@version)"`
	DryRun bool   `long:"dry-run" description:"Print actions without executing"`
}

func (c *depApproveCommand) Execute(_ []string) error {
	ctx := context.Background()
	mrs, err := selectGroup(ctx, c.runner, c.scopeFlags, c.Group)
	if err != nil {
		return err
	}

	client := gitlab.NewClient(c.runner)
	view := ui.New(c.inout.Stdout, mrs, false)
	applyAction(view, mrs, c.DryRun, "approve", func(mr types.MR) error {
		return client.ApproveMR(ctx, mr.Project, mr.IID)
	})
	return nil
}

type depMergeCommand struct {
	scopeFlags

	inout  *cli.ProcInout
	runner glab.Runner

	Group  string `long:"group" required:"true" description:"Group key (package@version)"`
	DryRun bool   `long:"dry-run" description:"Print actions without executing"`
	Method string `long:"method" default:"squash" description:"Merge method: merge, squash, or rebase"`
	// RequireChecks is a pointer because go-flags bool flags cannot default to
	// true; nil means unset, which this command treats as enabled.
	RequireChecks *bool `long:"require-checks" description:"Auto-merge when the pipeline succeeds (default true)"`
}

func (c *depMergeCommand) Execute(_ []string) error {
	if c.Method != "merge" && c.Method != "squash" && c.Method != "rebase" {
		return fmt.Errorf("%w %q (must be 'merge', 'squash', or 'rebase')", errInvalidMergeMethod, c.Method)
	}

	requireChecks := c.RequireChecks == nil || *c.RequireChecks

	ctx := context.Background()
	mrs, err := selectGroup(ctx, c.runner, c.scopeFlags, c.Group)
	if err != nil {
		return err
	}

	// With --require-checks, GitLab merges each MR once its pipeline succeeds
	// (native auto-merge) rather than this tool gating it.
	var successDetails []string
	if requireChecks {
		successDetails = []string{"auto-merge when pipeline succeeds"}
	}

	client := gitlab.NewClient(c.runner)
	view := ui.New(c.inout.Stdout, mrs, false)
	applyAction(view, mrs, c.DryRun, "merge", func(mr types.MR) error {
		return client.MergeMR(ctx, mr.Project, mr.IID, c.Method, requireChecks)
	}, successDetails...)
	return nil
}

// applyAction runs action against each MR, printing a consistent dry-run,
// success, or per-MR error line and continuing past individual failures.
func applyAction(
	view *ui.UI,
	mrs []types.MR,
	dryRun bool,
	verb string,
	action func(types.MR) error,
	successDetails ...string,
) {
	for _, mergeRequest := range mrs {
		if dryRun {
			view.PrintAction("[dry-run] "+verb, mergeRequest)
			continue
		}
		if err := action(mergeRequest); err != nil {
			view.PrintError(verb, mergeRequest, err)
			continue
		}
		view.PrintAction(verb, mergeRequest, successDetails...)
	}
}

// selectGroup searches for MRs in the given scope, groups them by
// package@version, and returns the MRs in the requested group. It replaces the
// upstream disk cache: the group is recomputed on each invocation.
func selectGroup(ctx context.Context, runner glab.Runner, scope scopeFlags, key string) ([]types.MR, error) {
	params, patterns := scope.resolve(ctx, runner)
	mrs, err := gitlab.NewClient(runner).SearchMRs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("search MRs: %w", err)
	}
	groups := gitlab.GroupMRs(mrs, patterns)
	selected, ok := groups[key]
	if !ok {
		return nil, fmt.Errorf("%w: %q", errGroupNotFound, key)
	}
	return selected, nil
}
