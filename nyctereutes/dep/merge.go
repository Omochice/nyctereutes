package dep

import (
	"context"
	"errors"
	"fmt"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/dep/gitlab"
	"github.com/Omochice/nyctereutes/internal/dep/types"
	"github.com/Omochice/nyctereutes/internal/dep/ui"
	"github.com/Omochice/nyctereutes/internal/glab"
)

var errInvalidMergeMethod = errors.New("invalid merge method")

type mergeCommand struct {
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

func (c *mergeCommand) Execute(_ []string) error {
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
	return applyAction(view, mrs, c.DryRun, "merge", func(mr types.MR) error {
		return client.MergeMR(ctx, mr.Project, mr.IID, c.Method, requireChecks)
	}, successDetails...)
}
