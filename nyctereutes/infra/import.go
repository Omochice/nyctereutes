package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var (
	errImportNeedsTarget   = errors.New("import requires at least one <owner/project>")
	errSomeImportsFailed   = errors.New("some projects could not be imported")
	errSomeSchedulesUnread = errors.New("some projects were exported without their pipeline schedules")
)

type importCommand struct {
	inout     *cli.ProcInout
	runner    glab.Runner
	schemaRef string
}

// Fetches each named project's basic settings from GitLab and writes them as
// YAML manifests to stdout, separated by "---" and each headed by the schema
// modeline, so an editor validates and completes the export as it is edited.
// Missing projects are reported on stderr and skipped; the run exits non-zero
// when any project failed, and also when a project was exported with its
// schedules left undescribed.
func (c *importCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errImportNeedsTarget
	}

	ctx := context.Background()
	client := repository.NewClient(c.runner)

	emitted, undescribed := 0, 0
	for _, target := range args {
		data, unread := c.document(ctx, client, target)
		if unread {
			undescribed++
		}
		if data == nil {
			continue
		}
		if emitted > 0 {
			_, _ = fmt.Fprintln(c.inout.Stdout, "---")
		}
		// An editor reads the modeline out of one document's own leading
		// comments and does not carry it across a separator, so every document
		// in the stream needs its own.
		_, _ = io.WriteString(c.inout.Stdout, manifest.SchemaModeline(c.schemaRef))
		_, _ = c.inout.Stdout.Write(data)
		emitted++
	}

	// A run can fall short both ways at once, and each names a different thing
	// for the reader to go and look at, so neither may mask the other. Joining
	// also keeps both answerable through errors.Is.
	var failed, partial error
	// Every target that did not emit a document failed one of the checks above.
	if emitted < len(args) {
		failed = fmt.Errorf("%w: %d of %d", errSomeImportsFailed, len(args)-emitted, len(args))
	}
	// A document that leaves the schedules undescribed is correct but partial,
	// and a caller redirecting stdout to a file would otherwise see nothing
	// wrong with it.
	if undescribed > 0 {
		partial = fmt.Errorf("%w: %d of %d", errSomeSchedulesUnread, undescribed, len(args))
	}
	return errors.Join(failed, partial)
}

// Builds one target's manifest document, or nil when it cannot be described.
// Each way of failing is reported here and answered with no document, which is
// what keeps one target's failure from ending the run. The second result says
// the document was built without the project's schedules, which is a partial
// description rather than a failed one.
func (c *importCommand) document(
	ctx context.Context, client *repository.Client, target string,
) (data []byte, undescribed bool) {
	owner, name, ok := splitTarget(target)
	if !ok {
		c.reportf("skip %q: not in <owner/project> form\n", target)
		return nil, false
	}
	state, err := client.FetchRepository(ctx, owner, name)
	if err != nil {
		// The error already carries "fetch project <owner>/<name>" context.
		c.reportf("%v\n", err)
		return nil, false
	}
	if state.IsNew {
		c.reportf("project %s not found on GitLab\n", target)
		return nil, false
	}
	// An exported document describes the whole project, so its schedules are
	// always read; a command that only reconciles some settings need not. A read
	// that fails costs them alone, because ending the export here would throw
	// away every setting already read over one child resource.
	schedules, err := client.FetchSchedules(ctx, owner, name)
	if err != nil {
		c.reportf("%v\n", err)
		undescribed = true
	} else {
		state.PipelineSchedules = schedules
	}
	data, err = manifest.Marshal(repository.ToManifest(state))
	if err != nil {
		c.reportf("marshal %s: %v\n", target, err)
		return nil, false
	}
	return data, undescribed
}

// Writes a diagnostic to stderr, whether it reports a failure or a limit on
// what could be described. A write failure there is nothing the command can act
// on: the stream it would use to say so is the one that just failed.
func (c *importCommand) reportf(format string, msgArgs ...any) {
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
