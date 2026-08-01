package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var (
	errImportNeedsTarget = errors.New("import requires at least one <owner/project>")
	errSomeImportsFailed = errors.New("some projects could not be imported")
)

type importCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner
}

// Fetches each named project's basic settings from GitLab and writes them as
// YAML manifests to stdout, separated by "---". Missing projects are reported on
// stderr and skipped; the run exits non-zero when any project failed.
func (c *importCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errImportNeedsTarget
	}

	ctx := context.Background()
	client := repository.NewClient(c.runner)

	emitted := 0
	for _, target := range args {
		data := c.document(ctx, client, target)
		if data == nil {
			continue
		}
		if emitted > 0 {
			_, _ = fmt.Fprintln(c.inout.Stdout, "---")
		}
		_, _ = c.inout.Stdout.Write(data)
		emitted++
	}

	// Every target that did not emit a document failed one of the checks above.
	if emitted < len(args) {
		return fmt.Errorf("%w: %d of %d", errSomeImportsFailed, len(args)-emitted, len(args))
	}
	return nil
}

// Builds one target's manifest document, or nil when it cannot be described.
// Each way of failing is reported here and answered with no document, which is
// what keeps one target's failure from ending the run.
func (c *importCommand) document(ctx context.Context, client *repository.Client, target string) []byte {
	owner, name, ok := splitTarget(target)
	if !ok {
		c.failf("skip %q: not in <owner/project> form\n", target)
		return nil
	}
	state, err := client.FetchRepository(ctx, owner, name)
	if err != nil {
		// The error already carries "fetch project <owner>/<name>" context.
		c.failf("%v\n", err)
		return nil
	}
	if state.IsNew {
		c.failf("project %s not found on GitLab\n", target)
		return nil
	}
	// An exported document describes the whole project, so its schedules are
	// always read; a command that only reconciles some settings need not.
	schedules, err := client.FetchSchedules(ctx, owner, name)
	if err != nil {
		c.failf("%v\n", err)
		return nil
	}
	state.PipelineSchedules = schedules
	data, err := manifest.Marshal(repository.ToManifest(state))
	if err != nil {
		c.failf("marshal %s: %v\n", target, err)
		return nil
	}
	return data
}

// Reports a diagnostic on stderr, where a write failure is nothing the command
// can act on: the stream it would use to say so is the one that just failed.
func (c *importCommand) failf(format string, msgArgs ...any) {
	_, _ = fmt.Fprintf(c.inout.Stderr, format, msgArgs...)
}

// Splits an "<owner>/<project>" target into its owner (which may itself be a
// nested group path) and the trailing project name. A leading or doubled slash
// is rejected so a malformed target is reported as such rather than encoded into
// a bogus path that GitLab answers with a misleading 404.
func splitTarget(target string) (owner, name string, ok bool) {
	i := strings.LastIndex(target, "/")
	if i <= 0 || i == len(target)-1 {
		return "", "", false
	}
	owner, name = target[:i], target[i+1:]
	if strings.HasPrefix(owner, "/") || strings.HasSuffix(owner, "/") {
		return "", "", false
	}
	return owner, name, true
}
