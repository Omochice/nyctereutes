// The "infra" subcommand tree, which manages GitLab project settings through
// YAML manifests.
package infra

import (
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
