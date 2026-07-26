// The "dep" subcommand tree, which manages dependency-update merge requests.
package dep

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/dep/config"
	"github.com/Omochice/nyctereutes/internal/dep/gitlab"
	"github.com/Omochice/nyctereutes/internal/dep/tui"
	"github.com/Omochice/nyctereutes/internal/dep/types"
	"github.com/Omochice/nyctereutes/internal/dep/ui"
	"github.com/Omochice/nyctereutes/internal/glab"
)

var (
	errGroupNotFound     = errors.New("group not found")
	errSomeActionsFailed = errors.New("some operations failed")
)

// The search-scope flags shared by list, approve and merge.
// Repo and Author are pointers so an explicit (even empty) flag can be told
// apart from "not specified", in which case config or a default is used.
type scopeFlags struct {
	Repo      *string `short:"R" long:"repo" description:"Target project(s) (GROUP/PROJECT), comma-separated"`
	Author    *string `long:"author" description:"MR author username; --author= matches any (default: Renovate bot)"`
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

// The command tree go-flags parses "dep" and its subcommands into.
type Command struct {
	scopeFlags

	inout  *cli.ProcInout
	runner glab.Runner
	// Starts the interactive TUI; it is a field so tests can substitute a fake
	// instead of driving a real terminal program.
	launch func(tui.Model) error

	List    *listCommand    `command:"list" description:"list dependency MRs"`
	Approve *approveCommand `command:"approve" description:"bulk approve a group of MRs"`
	Merge   *mergeCommand   `command:"merge" description:"bulk merge a group of MRs"`
}

// Builds the tree with every subcommand wired to the given streams and glab
// runner, so a caller can inject a fake runner instead of the real CLI.
func New(inout *cli.ProcInout, runner glab.Runner) *Command {
	return &Command{
		inout:  inout,
		runner: runner,
		launch: func(model tui.Model) error {
			return tui.Run(model, inout.Stdin, inout.Stdout)
		},
		List:    &listCommand{inout: inout, runner: runner},
		Approve: &approveCommand{inout: inout, runner: runner},
		Merge:   &mergeCommand{inout: inout, runner: runner},
	}
}

// Runs when "dep" is invoked with no subcommand: it searches the configured
// scope and opens the interactive TUI, or prints the empty message and exits
// without launching when nothing matches.
func (c *Command) Execute(_ []string) error {
	ctx := context.Background()
	params, patterns := c.resolve(ctx, c.runner)

	client := gitlab.NewClient(c.runner)
	mrs, err := client.SearchMRs(ctx, params)
	if err != nil {
		return fmt.Errorf("search MRs: %w", err)
	}
	if len(mrs) == 0 {
		_, _ = fmt.Fprintln(c.inout.Stdout, "No dependency MRs found")
		return nil
	}
	model := tui.New(
		client, mrs,
		tui.WithGroupKey(gitlab.GroupKeyFunc(patterns)),
		tui.WithOpen(func(mr types.MR) error {
			_, err := c.runner.Run(ctx, "mr", "view", strconv.Itoa(mr.IID), "-R", mr.Project, "--web")
			if err != nil {
				return fmt.Errorf("open MR in browser: %w", err)
			}
			return nil
		}),
		tui.WithRefresh(func() ([]types.MR, error) {
			refreshed, err := client.SearchMRs(ctx, params)
			if err != nil {
				return nil, fmt.Errorf("search MRs: %w", err)
			}
			return refreshed, nil
		}),
	)
	return c.launch(model)
}

// Runs action against each MR, printing a consistent dry-run, success, or
// per-MR error line and continuing past individual failures. Returns a non-nil
// error when any MR failed so the command exits non-zero.
func applyAction(
	view *ui.UI,
	mrs []types.MR,
	dryRun bool,
	verb string,
	action func(types.MR) error,
	successDetails ...string,
) error {
	failures := 0
	for _, mergeRequest := range mrs {
		if dryRun {
			view.PrintAction("[dry-run] "+verb, mergeRequest)
			continue
		}
		if err := action(mergeRequest); err != nil {
			view.PrintError(verb, mergeRequest, err)
			failures++
			continue
		}
		view.PrintAction(verb, mergeRequest, successDetails...)
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d of %d", errSomeActionsFailed, failures, len(mrs))
	}
	return nil
}

// Searches for MRs in the given scope, groups them by package@version, and
// returns the MRs in the requested group. It replaces the upstream disk cache:
// the group is recomputed on each invocation.
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
