package repository

import (
	"context"
	"encoding/json"
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

// Signals a project the manifest declares but GitLab lacks; this slice applies
// only updates, so a create is reported rather than performed.
var errCreateUnsupported = errors.New("creating a project is not supported yet")

// Translates one change into its glab call. A create is reported unsupported;
// archived is toggled through its own endpoint; every other field is a scalar
// PUT.
func (a *Applier) applyChange(ctx context.Context, change Change) error {
	if change.Type == ChangeCreate {
		return fmt.Errorf("%w: %s", errCreateUnsupported, change.Name)
	}
	switch change.Field {
	case fieldArchived:
		archived, ok := change.NewValue.(bool)
		if !ok {
			return fmt.Errorf("%w: archived got %T", errUnexpectedValueType, change.NewValue)
		}
		return a.setArchived(ctx, change.Name, archived)
	case fieldCICatalog:
		enabled, ok := change.NewValue.(bool)
		if !ok {
			return fmt.Errorf("%w: ci_catalog got %T", errUnexpectedValueType, change.NewValue)
		}
		return a.setCatalogResource(ctx, change.Name, enabled)
	case fieldTopics:
		topics, ok := change.NewValue.([]string)
		if !ok {
			return fmt.Errorf("%w: topics got %T", errUnexpectedValueType, change.NewValue)
		}
		return a.applyTopics(ctx, change.Name, topics)
	default:
		return a.putField(ctx, change.Name, change.Field, fmt.Sprintf("%v", change.NewValue))
	}
}

// Archives or unarchives a project. Unlike other settings the archived state is
// a POST to a dedicated action endpoint rather than a field on the project.
func (a *Applier) setArchived(ctx context.Context, project string, archived bool) error {
	action := "unarchive"
	if archived {
		action = "archive"
	}
	_, err := a.writer.Run(ctx, "api", "projects/"+glab.EncodePath(project)+"/"+action, "--method", "POST")
	return wrapWrite(err, project, fieldArchived)
}

// The GraphQL mutations that publish a project to the CI/CD Catalog and remove
// it. Unlike the REST settings, catalog membership is not a field on the
// project, so it is toggled by these dedicated mutations rather than a PUT.
const (
	catalogResourceCreateMutation = `mutation($projectPath: ID!) {
  catalogResourcesCreate(input: { projectPath: $projectPath }) { errors }
}`
	catalogResourceDestroyMutation = `mutation($projectPath: ID!) {
  catalogResourcesDestroy(input: { projectPath: $projectPath }) { errors }
}`
)

// Signals a GraphQL mutation that returned a 200 carrying a non-empty errors
// payload; glab reports success for such a response, so the payload must be
// inspected rather than the exit status alone.
var errCatalogMutationFailed = errors.New("catalog mutation failed")

// Publishes a project to the CI/CD Catalog or removes it. The projectPath is
// the raw namespace-qualified path GraphQL addresses a project by, so unlike
// the REST endpoints it is not percent-encoded.
func (a *Applier) setCatalogResource(ctx context.Context, project string, enabled bool) error {
	query, field := catalogResourceDestroyMutation, "catalogResourcesDestroy"
	if enabled {
		query, field = catalogResourceCreateMutation, "catalogResourcesCreate"
	}
	out, err := a.writer.Run(ctx, "api", "graphql",
		"-f", "query="+query,
		"-f", "projectPath="+project)
	if err != nil {
		return wrapWrite(err, project, fieldCICatalog)
	}
	return wrapWrite(catalogMutationErrors(out, field), project, fieldCICatalog)
}

// Reports a mutation whose payload carries errors. A GraphQL mutation answers
// with HTTP 200 even when it refuses the operation, listing the reasons under
// the mutation field's errors array, so a green exit is not proof of success.
func catalogMutationErrors(out []byte, field string) error {
	var resp struct {
		Data map[string]struct {
			Errors []string `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("parse catalog mutation response: %w", err)
	}
	if payload, ok := resp.Data[field]; ok && len(payload.Errors) > 0 {
		return fmt.Errorf("%w: %s", errCatalogMutationFailed, strings.Join(payload.Errors, "; "))
	}
	return nil
}

// Names the field and project on a failed write so the aggregated report says
// what could not be applied, prepending a per-class hint while keeping the
// sentinel in the chain for [errors.Is]. A nil error passes through unchanged.
func wrapWrite(err error, project, field string) error {
	if err == nil {
		return nil
	}
	if hint := writeHint(err); hint != "" {
		err = fmt.Errorf("%s: %w", hint, err)
	}
	return fmt.Errorf("apply %s on %s: %w", field, project, err)
}

// The hint names the likely cause of a classified failure so the operator
// reading the aggregated report knows whether to fix a token, a path, or a
// value; "" when the failure was not classified.
func writeHint(err error) string {
	switch {
	case errors.Is(err, glab.ErrForbidden):
		return "permission denied; check the token has the Maintainer or Owner role"
	case errors.Is(err, glab.ErrNotFound):
		return "project not found; it may have been removed or renamed"
	case errors.Is(err, glab.ErrValidation):
		return "GitLab rejected the value"
	}
	return ""
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

// Updates one scalar project setting with a PUT to the projects endpoint. The
// plan field names the change to GitLab's API parameter for the request but to
// the manifest field in any error, so a failure lines up with the printed plan.
func (a *Applier) putField(ctx context.Context, project, planField, value string) error {
	_, err := a.writer.Run(
		ctx,
		"api", "projects/"+glab.EncodePath(project),
		"--method", "PUT",
		"-f", apiParam(planField)+"="+value,
	)
	return wrapWrite(err, project, planField)
}

// Replaces a project's whole topic list. GitLab's projects PUT performs a full
// replacement, so the desired list is sent verbatim and an empty list clears
// every topic. A JSON body carries the array because a form field cannot send
// an empty list.
func (a *Applier) applyTopics(ctx context.Context, project string, topics []string) error {
	if topics == nil {
		topics = []string{}
	}
	body, err := json.Marshal(map[string]any{fieldTopics: topics})
	if err != nil {
		return fmt.Errorf("marshal topics: %w", err)
	}
	_, err = a.writer.RunWithStdin(
		ctx, body,
		"api", "projects/"+glab.EncodePath(project),
		"--method", "PUT",
		"--header", "Content-Type: application/json",
		"--input", "-",
	)
	return wrapWrite(err, project, fieldTopics)
}
