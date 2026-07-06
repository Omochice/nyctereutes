package nyctereutes

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var (
	errApplyNeedsPath = errors.New("apply requires at least one <path>")
	errApplyNoWriter  = errors.New("runner cannot write to GitLab")
	errApplyFailed    = errors.New("apply failed")
)

type infraApplyCommand struct {
	inout  *cli.ProcInout
	writer repository.ProjectWriter

	AutoApprove bool `long:"auto-approve" description:"apply without the confirmation prompt"`
}

// One project's planned changes, kept together so the whole plan can be shown
// before a single confirmation covers every change.
type repoPlan struct {
	name    string
	changes []repository.Change
}

// Applies each manifest's declared state to its live GitLab project. The full
// plan is shown first; unless --auto-approve is given the user must confirm
// before any write. Read, parse and fetch problems are reported and counted
// rather than fatal, and a non-zero exit follows any failure.
func (c *infraApplyCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errApplyNeedsPath
	}
	if c.writer == nil {
		return errApplyNoWriter
	}

	ctx := context.Background()
	plans, failures := c.buildPlans(ctx, repository.NewClient(c.writer), args)

	if len(plans) == 0 {
		if failures == 0 {
			_, _ = fmt.Fprintln(c.inout.Stdout, "No changes.")
		}
		return c.result(failures)
	}

	c.printPlans(plans, wantsColor(c.inout.Stdout))
	if c.AutoApprove || c.confirm() {
		failures += c.applyPlans(ctx, plans)
	} else {
		_, _ = fmt.Fprintln(c.inout.Stdout, "Apply canceled.")
	}
	return c.result(failures)
}

// Turns an aggregated failure count into the command's exit outcome.
func (c *infraApplyCommand) result(failures int) error {
	if failures > 0 {
		return fmt.Errorf("%w: %d problem(s)", errApplyFailed, failures)
	}
	return nil
}

// Diffs every declared project against its live state, returning the projects
// that drift together with the count of read/parse/fetch problems.
func (c *infraApplyCommand) buildPlans(
	ctx context.Context, client *repository.Client, args []string,
) (plans []repoPlan, failures int) {
	for _, path := range args {
		files, err := manifestFiles(path)
		if err != nil {
			_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", err)
			failures++
			continue
		}
		for _, file := range files {
			repos, failed := readManifestFile(c.inout.Stderr, file)
			failures += failed
			for _, repo := range repos {
				state, err := client.FetchRepository(ctx, repo.Metadata.Owner, repo.Metadata.Name)
				if err != nil {
					_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", err)
					failures++
					continue
				}
				changes := repository.Diff(repo, state)
				if len(changes) == 0 {
					continue
				}
				plans = append(plans, repoPlan{
					name:    repo.Metadata.Owner + "/" + repo.Metadata.Name,
					changes: changes,
				})
			}
		}
	}
	return plans, failures
}

// Prints the pending changes in the same layout as infra plan.
func (c *infraApplyCommand) printPlans(plans []repoPlan, colorize bool) {
	for _, plan := range plans {
		printChanges(c.inout.Stdout, plan.name, plan.changes, colorize)
	}
}

// Prints the plan, asks once for confirmation, and reports whether the user
// approved. A non-yes answer, EOF or unreadable input all count as a decline so
// a non-interactive run without --auto-approve never writes by accident.
func (c *infraApplyCommand) confirm() bool {
	_, _ = fmt.Fprint(c.inout.Stdout, "Apply these changes? [y/N] ")
	scanner := bufio.NewScanner(c.inout.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

// Applies every plan, reporting each failed change and returning how many
// changes could not be applied.
func (c *infraApplyCommand) applyPlans(ctx context.Context, plans []repoPlan) (failures int) {
	applier := repository.NewApplier(c.writer)
	for _, plan := range plans {
		for _, result := range applier.Apply(ctx, plan.changes) {
			if result.Err != nil {
				_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", result.Err)
				failures++
			}
		}
	}
	return failures
}
