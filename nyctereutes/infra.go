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
	fail := func(format string, msgArgs ...any) {
		_, _ = fmt.Fprintf(c.inout.Stderr, format, msgArgs...)
	}

	emitted := 0
	for _, target := range args {
		owner, name, ok := splitTarget(target)
		if !ok {
			fail("skip %q: not in <owner/project> form\n", target)
			continue
		}
		state, err := client.FetchRepository(ctx, owner, name)
		if err != nil {
			fail("fetch %s: %v\n", target, err)
			continue
		}
		if state.IsNew {
			fail("project %s not found on GitLab\n", target)
			continue
		}
		data, err := goyaml.Marshal(repository.ToManifest(state))
		if err != nil {
			fail("marshal %s: %v\n", target, err)
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

// Splits an "<owner>/<project>" target into its owner (which may itself be a
// nested group path) and the trailing project name.
func splitTarget(target string) (owner, name string, ok bool) {
	i := strings.LastIndex(target, "/")
	if i <= 0 || i == len(target)-1 {
		return "", "", false
	}
	return target[:i], target[i+1:], true
}
