package dep

import (
	"context"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/dep/gitlab"
	"github.com/Omochice/nyctereutes/internal/dep/types"
	"github.com/Omochice/nyctereutes/internal/dep/ui"
	"github.com/Omochice/nyctereutes/internal/glab"
)

type approveCommand struct {
	scopeFlags

	inout  *cli.ProcInout
	runner glab.Runner

	Group  string `long:"group" required:"true" description:"Group key (package@version)"`
	DryRun bool   `long:"dry-run" description:"Print actions without executing"`
}

func (c *approveCommand) Execute(_ []string) error {
	ctx := context.Background()
	mrs, err := selectGroup(ctx, c.runner, c.scopeFlags, c.Group)
	if err != nil {
		return err
	}

	client := gitlab.NewClient(c.runner)
	view := ui.New(c.inout.Stdout, mrs, false)
	return applyAction(view, mrs, c.DryRun, "approve", func(mr types.MR) error {
		return client.ApproveMR(ctx, mr.Project, mr.IID)
	})
}
