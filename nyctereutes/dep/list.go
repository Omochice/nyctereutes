package dep

import (
	"context"
	"fmt"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/dep/gitlab"
	"github.com/Omochice/nyctereutes/internal/dep/ui"
	"github.com/Omochice/nyctereutes/internal/glab"
)

type listCommand struct {
	scopeFlags

	inout  *cli.ProcInout
	runner glab.Runner

	Group bool `long:"group" description:"Group MRs by package@version"`
	JSON  bool `long:"json" description:"Output as JSON"`
}

func (c *listCommand) Execute(_ []string) error {
	ctx := context.Background()
	params, patterns := c.resolve(ctx, c.runner)

	mrs, err := gitlab.NewClient(c.runner).SearchMRs(ctx, params)
	if err != nil {
		return fmt.Errorf("search MRs: %w", err)
	}
	// In JSON mode an empty result still has to be valid JSON for machine
	// consumers, so only the human-readable path prints a message here.
	if len(mrs) == 0 && !c.JSON {
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
