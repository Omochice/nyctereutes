package nyctereutes

import (
	"errors"
	"fmt"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

var (
	errValidateNeedsPath = errors.New("validate requires at least one <path>")
	errInvalidManifests  = errors.New("validation failed")
)

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
			parsed, failed := readManifestFile(c.inout.Stderr, file)
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
