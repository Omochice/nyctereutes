package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// A glab runner that can also stream a request body on stdin. The stdin variant
// exists for the topics PUT, whose empty-list-clears-all semantics a form field
// cannot express.
type ProjectWriter interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
	RunWithStdin(ctx context.Context, body []byte, args ...string) ([]byte, error)
}

// Applies planned [Change]s to GitLab projects through the glab CLI.
type Applier struct {
	writer ProjectWriter
}

// Builds an Applier that writes through writer.
func NewApplier(writer ProjectWriter) *Applier {
	return &Applier{writer: writer}
}

// The outcome of applying one [Change].
type ApplyResult struct {
	Change Change
	Err    error
}

// Applies each change in order, one result per change. A failed change is
// recorded and the rest still run, mirroring how plan and import aggregate
// rather than stop at the first problem.
func (a *Applier) Apply(ctx context.Context, changes []Change) []ApplyResult {
	results := make([]ApplyResult, 0, len(changes))
	for _, change := range changes {
		results = append(results, ApplyResult{Change: change, Err: a.applyChange(ctx, change)})
	}
	return results
}

// Signals a Change whose NewValue does not hold the type its field requires.
var errUnexpectedValueType = errors.New("change value has unexpected type")

// Translates one change into its glab call. Archived is toggled through its own
// endpoint; every other field is a scalar PUT.
func (a *Applier) applyChange(ctx context.Context, change Change) error {
	if change.Field == fieldArchived {
		archived, ok := change.NewValue.(bool)
		if !ok {
			return fmt.Errorf("%w: archived got %T", errUnexpectedValueType, change.NewValue)
		}
		return a.setArchived(ctx, change.Name, archived)
	}
	return a.putField(ctx, change.Name, apiParam(change.Field), fmt.Sprintf("%v", change.NewValue))
}

// Archives or unarchives a project. Unlike other settings the archived state is
// a POST to a dedicated action endpoint rather than a field on the project.
func (a *Applier) setArchived(ctx context.Context, project string, archived bool) error {
	action := "unarchive"
	if archived {
		action = "archive"
	}
	_, err := a.writer.Run(ctx, "api", "projects/"+glab.EncodePath(project)+"/"+action, "--method", "POST")
	return err
}

// Maps a plan field name to the GitLab API parameter that carries it. A
// features.<key> field becomes <key>_access_level; every other field already
// matches its API name. GitLab exposes CI under builds_access_level, so the
// friendlier "ci" key is the one exception to the mechanical mapping.
func apiParam(field string) string {
	key, ok := strings.CutPrefix(field, "features.")
	if !ok {
		return field
	}
	if key == "ci" {
		key = "builds"
	}
	return key + "_access_level"
}

// Updates one scalar project setting with a PUT to the projects endpoint.
func (a *Applier) putField(ctx context.Context, project, field, value string) error {
	_, err := a.writer.Run(
		ctx,
		"api", "projects/"+glab.EncodePath(project),
		"--method", "PUT",
		"-f", field+"="+value,
	)
	return err
}
