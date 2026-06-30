package nyctereutes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	goyaml "github.com/goccy/go-yaml"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var (
	errImportNeedsTarget = errors.New("import requires at least one <owner/project>")
	errSomeImportsFailed = errors.New("some projects could not be imported")
)

type infraCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner

	Import *infraImportCommand `command:"import" description:"export GitLab project settings as YAML"`
}

func newInfraCommand(inout *cli.ProcInout, runner glab.Runner) *infraCommand {
	return &infraCommand{
		inout:  inout,
		runner: runner,
		Import: &infraImportCommand{inout: inout, runner: runner},
	}
}

type infraImportCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner
}

// Fetches each named project's basic settings from GitLab and writes them as
// YAML manifests to stdout, separated by "---". Missing projects are reported on
// stderr and skipped; the run exits non-zero when any project failed.
func (c *infraImportCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errImportNeedsTarget
	}

	ctx := context.Background()
	client := repository.NewClient(c.runner)
	failures := 0
	emitted := 0
	for _, target := range args {
		owner, name, ok := splitTarget(target)
		if !ok {
			_, _ = fmt.Fprintf(c.inout.Stderr, "skip %q: not in <owner/project> form\n", target)
			failures++
			continue
		}
		state, err := client.FetchRepository(ctx, owner, name)
		if err != nil {
			_, _ = fmt.Fprintf(c.inout.Stderr, "fetch %s: %v\n", target, err)
			failures++
			continue
		}
		if state.IsNew {
			_, _ = fmt.Fprintf(c.inout.Stderr, "project %s not found on GitLab\n", target)
			failures++
			continue
		}
		data, err := goyaml.Marshal(repository.ToManifest(state))
		if err != nil {
			_, _ = fmt.Fprintf(c.inout.Stderr, "marshal %s: %v\n", target, err)
			failures++
			continue
		}
		if emitted > 0 {
			_, _ = fmt.Fprintln(c.inout.Stdout, "---")
		}
		_, _ = fmt.Fprint(c.inout.Stdout, string(data))
		emitted++
	}

	if failures > 0 {
		return fmt.Errorf("%w: %d of %d", errSomeImportsFailed, failures, len(args))
	}
	return nil
}

// Splits an "<owner>/<project>" target into its owner (which may itself be a
// nested group path) and the trailing project name.
func splitTarget(target string) (owner, name string, ok bool) {
	i := strings.LastIndex(target, "/")
	if i <= 0 || i == len(target)-1 {
		return "", "", false
	}
	return target[:i], target[i+1:], true
}
