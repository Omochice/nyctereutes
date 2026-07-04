package nyctereutes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

var (
	errImportNeedsTarget = errors.New("import requires at least one <owner/project>")
	errSomeImportsFailed = errors.New("some projects could not be imported")
	errValidateNeedsPath = errors.New("validate requires at least one <path>")
	errInvalidManifests  = errors.New("validation failed")
	errNoManifestsFound  = errors.New("no .yaml/.yml files in directory")
	errPlanNeedsPath     = errors.New("plan requires at least one <path>")
	errPlanDrift         = errors.New("changes detected")
	errPlanFailed        = errors.New("plan failed")
)

type infraCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner

	Import   *infraImportCommand   `command:"import" description:"export GitLab project settings as YAML"`
	Validate *infraValidateCommand `command:"validate" description:"validate manifest YAML files against the schema"`
	Plan     *infraPlanCommand     `command:"plan" description:"show drift between manifests and live GitLab state"`
}

func newInfraCommand(inout *cli.ProcInout, runner glab.Runner) *infraCommand {
	return &infraCommand{
		inout:    inout,
		runner:   runner,
		Import:   &infraImportCommand{inout: inout, runner: runner},
		Validate: &infraValidateCommand{inout: inout},
		Plan:     &infraPlanCommand{inout: inout, runner: runner},
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
			// The error already carries "fetch project <owner>/<name>" context.
			fail("%v\n", err)
			continue
		}
		if state.IsNew {
			fail("project %s not found on GitLab\n", target)
			continue
		}
		data, err := manifest.Marshal(repository.ToManifest(state))
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

type infraValidateCommand struct {
	inout *cli.ProcInout
}

// Validates manifest YAML files against the schema. Every problem is reported
// on stderr with its file and document position before the run fails, so one
// broken document does not hide the rest; a fully valid run summarizes the
// documents on stdout.
func (c *infraValidateCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errValidateNeedsPath
	}

	var repos []*manifest.Repository
	failures := 0
	for _, path := range args {
		files, err := manifestFiles(path)
		if err != nil {
			_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", err)
			failures++
			continue
		}
		for _, file := range files {
			parsed, failed := c.validateFile(file)
			repos = append(repos, parsed...)
			failures += failed
		}
	}

	if failures > 0 {
		return fmt.Errorf("%w: %d problem(s)", errInvalidManifests, failures)
	}
	_, _ = fmt.Fprintf(c.inout.Stdout, "Valid: %d repositories\n", len(repos))
	for _, repo := range repos {
		_, _ = fmt.Fprintf(c.inout.Stdout, "  - %s/%s\n", repo.Metadata.Owner, repo.Metadata.Name)
	}
	return nil
}

// Reports every schema violation in one file to stderr and returns the
// documents that validated along with the number of problems found.
func (c *infraValidateCommand) validateFile(path string) ([]*manifest.Repository, int) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", err)
		return nil, 1
	}
	repos, errs := manifest.Parse(data)
	for _, parseErr := range errs {
		_, _ = fmt.Fprintf(c.inout.Stderr, "%s: %v\n", path, parseErr)
	}
	return repos, len(errs)
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

type infraPlanCommand struct {
	inout  *cli.ProcInout
	runner glab.Runner

	CI bool `long:"ci" description:"exit non-zero when any drift is detected"`
}

// Shows how each declared manifest differs from its live GitLab project. Every
// project's drift is printed under an "owner/name" header on stdout.
func (c *infraPlanCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errPlanNeedsPath
	}

	ctx := context.Background()
	client := repository.NewClient(c.runner)
	changed := 0
	failures := 0
	for _, path := range args {
		files, err := manifestFiles(path)
		if err != nil {
			return err
		}
		for _, file := range files {
			data, err := os.ReadFile(filepath.Clean(file))
			if err != nil {
				return err
			}
			repos, errs := manifest.Parse(data)
			for _, parseErr := range errs {
				_, _ = fmt.Fprintf(c.inout.Stderr, "%s: %v\n", file, parseErr)
				failures++
			}
			for _, repo := range repos {
				state, err := client.FetchRepository(ctx, repo.Metadata.Owner, repo.Metadata.Name)
				if err != nil {
					// One project's fetch failure must not hide the drift of
					// the others, so report it and move on.
					_, _ = fmt.Fprintf(c.inout.Stderr, "%v\n", err)
					failures++
					continue
				}
				changes := repository.Diff(repo, state)
				if len(changes) == 0 {
					continue
				}
				changed++
				_, _ = fmt.Fprintf(c.inout.Stdout, "%s/%s\n", repo.Metadata.Owner, repo.Metadata.Name)
				for _, change := range changes {
					_, _ = fmt.Fprintf(c.inout.Stdout, "  %s\n", change)
				}
			}
		}
	}

	if changed == 0 && failures == 0 {
		_, _ = fmt.Fprintln(c.inout.Stdout, "No changes.")
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d problem(s)", errPlanFailed, failures)
	}
	// Drift is reported, not an error, so a human run always succeeds; --ci
	// turns detected drift into a non-zero exit for pipeline gating.
	if c.CI && changed > 0 {
		return errPlanDrift
	}
	return nil
}
