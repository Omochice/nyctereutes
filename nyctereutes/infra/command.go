// The "infra" subcommand tree, which manages GitLab project settings through
// YAML manifests.
package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var errNoManifestsFound = errors.New("no .yaml/.yml files in directory")

// The command tree go-flags parses "infra" and its subcommands into. It has no
// Execute of its own: "infra" alone is a usage error, so the streams and the
// runner are held by the subcommands rather than here.
type Command struct {
	Import   *importCommand   `command:"import" description:"export GitLab project settings as YAML"`
	Validate *validateCommand `command:"validate" description:"validate manifest YAML files against the schema"`
	Plan     *planCommand     `command:"plan" description:"show drift between manifests and live GitLab state"`
	Apply    *applyCommand    `command:"apply" description:"apply manifests to live GitLab state"`
}

// Builds the tree with every subcommand wired to the given streams and glab
// runner, so a caller can inject a fake runner instead of the real CLI.
func New(inout *cli.ProcInout, runner glab.Runner) *Command {
	// apply needs to stream a request body for topics, which only the
	// stdin-capable runner provides; a runner without it leaves writer nil and
	// apply reports that it cannot write.
	writer, _ := runner.(repository.ProjectWriter)
	return &Command{
		Import:   &importCommand{inout: inout, runner: runner},
		Validate: &validateCommand{inout: inout},
		Plan:     &planCommand{inout: inout, runner: runner},
		Apply:    &applyCommand{inout: inout, writer: writer},
	}
}

// Reads and parses one manifest file, reporting an unreadable file or any
// parse error to stderr and returning the documents that parsed with the
// number of problems found. Validate and plan share it so the two commands
// read manifests identically.
func readManifestFile(stderr io.Writer, path string) ([]*manifest.Repository, int) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return nil, 1
	}
	repos, errs := manifest.Parse(data)
	for _, parseErr := range errs {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", path, parseErr)
	}
	return repos, len(errs)
}

// The changes one declared project needs to match its manifest, with the number
// of problems met on the way. Plan and apply share it so the plan shown before
// the confirmation prompt is built exactly as the one apply then performs.
func planRepo(
	ctx context.Context, client *repository.Client, stderr io.Writer, repo *manifest.Repository,
) ([]repository.Change, int) {
	state, failures := fetchState(ctx, client, stderr, repo)
	if state == nil {
		return nil, failures
	}
	changes := repository.Diff(repo, state)
	discloseDestroyedVariables(ctx, client, stderr, repo, changes)
	return changes, failures
}

// Names the variables each planned removal would destroy, so the operator
// approving one sees what goes with the schedule. The manifest holds no
// variables, which is what leaves the rest of the plan silent about them.
//
// Only a removal is read, so a plan that removes nothing pays nothing. A read
// that fails costs a warning and no more: the removal is still shown, because
// nothing about performing it depends on the answer. Keys are all that comes
// back, so the plan cannot disclose a value it was written to protect.
func discloseDestroyedVariables(
	ctx context.Context, client *repository.Client, stderr io.Writer,
	repo *manifest.Repository, changes []repository.Change,
) {
	for _, change := range changes {
		if change.Type != repository.ChangeDelete || change.Schedule == nil {
			continue
		}
		keys, err := client.ScheduleVariableKeys(
			ctx, repo.Metadata.Owner, repo.Metadata.Name, change.Schedule.ID,
		)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			continue
		}
		change.Schedule.VariableKeys = keys
	}
}

// Reads the live state one declared project is compared against. A failed read
// is written to stderr and counted rather than returned, the way
// [readManifestFile] treats an unparseable document, and a nil state is one
// nothing can be said about.
func fetchState(
	ctx context.Context, client *repository.Client, stderr io.Writer, repo *manifest.Repository,
) (*repository.CurrentState, int) {
	state, err := client.FetchRepository(ctx, repo.Metadata.Owner, repo.Metadata.Name)
	if err != nil {
		// The error already carries "fetch project <owner>/<name>" context.
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return nil, 1
	}
	// A manifest declaring no schedules says nothing about them, so the read is
	// not made and such a project stays at one request.
	if repo.Spec.PipelineSchedules == nil {
		return state, 0
	}
	schedules, err := client.FetchSchedules(ctx, repo.Metadata.Owner, repo.Metadata.Name)
	if err != nil {
		// The state still goes back, so a project whose schedules cannot be
		// described keeps the settings that can be.
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return state, 1
	}
	state.PipelineSchedules = schedules
	return state, 0
}

// Expands one path argument into the manifest files it names: a file is
// itself, a directory contributes its .yaml/.yml entries one level deep.
// Recursion is deliberately absent so an unrelated tree cannot leak in.
func manifestFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if !info.IsDir() {
		return []string{path}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Lower-cased so an "A.YAML" entry cannot silently escape validation.
		if ext := strings.ToLower(filepath.Ext(entry.Name())); ext != ".yaml" && ext != ".yml" {
			continue
		}
		files = append(files, filepath.Join(path, entry.Name()))
	}
	// Nothing to validate is a failure, not an empty success: a mistyped
	// directory would otherwise pass CI having checked nothing.
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoManifestsFound, path)
	}
	return files, nil
}
