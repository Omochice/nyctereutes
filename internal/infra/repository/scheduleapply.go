package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Signals a planned schedule change left unapplied because an earlier one on
// the same project failed. It is reported rather than dropped so the results
// account for every change the plan showed.
var errScheduleChangeSkipped = errors.New("not applied because an earlier pipeline schedule deletion failed")

// Signals a change whose values do not hold the schedule types the field
// requires, which would mean the diff and the apply disagree.
var errNotASchedule = errors.New("change does not carry a pipeline schedule")

// Applies one whole-schedule change. GitLab addresses a schedule through its
// own endpoints rather than as a field on the project, and by an id no manifest
// holds, so an update and a delete take the id the live read put on the change.
func (a *Applier) applySchedule(ctx context.Context, change Change) error {
	endpoint := "projects/" + glab.EncodePath(change.Name) + "/pipeline_schedules"
	switch change.Type {
	case ChangeDelete:
		live, ok := change.OldValue.(LiveSchedule)
		if !ok {
			return fmt.Errorf("%w: delete got %T", errNotASchedule, change.OldValue)
		}
		return a.runSchedule(ctx, apiCommand, fmt.Sprintf("%s/%d", endpoint, live.ID), methodFlag, "DELETE")
	case ChangeCreate:
		desired, ok := change.NewValue.(manifest.PipelineSchedule)
		if !ok {
			return fmt.Errorf("%w: create got %T", errNotASchedule, change.NewValue)
		}
		return a.createSchedule(ctx, endpoint, desired)
	case ChangeUpdate:
		desired, ok := change.NewValue.(manifest.PipelineSchedule)
		live, liveOK := change.OldValue.(LiveSchedule)
		if !ok || !liveOK {
			return fmt.Errorf("%w: update got %T and %T", errNotASchedule, change.OldValue, change.NewValue)
		}
		return a.updateSchedule(ctx, endpoint, desired, live)
	default:
		return fmt.Errorf("%w: %s", errNotASchedule, change.Type)
	}
}

// The form fields describing a schedule. active is always sent: omitting it
// would let GitLab apply its create default and quietly start a schedule the
// manifest declares as paused.
func scheduleForm(desired manifest.PipelineSchedule) []string {
	return []string{
		"-f", "description=" + desired.Description,
		"-f", "ref=" + string(desired.Ref),
		"-f", "cron=" + desired.Cron,
		"-f", "cron_timezone=" + desired.CronTimezone,
		"-f", "active=" + strconv.FormatBool(desired.Active),
	}
}

// Names the schedule and project on a failed write. A 403 gets its own message:
// the generic hint blames a missing role, but a schedule refuses an update from
// anyone who does not own it however much project access they hold.
func wrapScheduleWrite(err error, change Change) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, glab.ErrForbidden) {
		err = fmt.Errorf("the schedule is owned by another user; only its owner or an instance administrator "+
			"can change it: %w", err)
	}
	return fmt.Errorf("apply pipeline schedule %q on %s: %w", scheduleName(change), change.Name, err)
}

// The description a change is about, taken from whichever side carries it.
func scheduleName(change Change) string {
	if desired, ok := change.NewValue.(manifest.PipelineSchedule); ok {
		return desired.Description
	}
	if live, ok := change.OldValue.(LiveSchedule); ok {
		return live.Description
	}
	return ""
}

// The glab subcommand and flag the schedule endpoints share, named once so they
// cannot be spelled differently across the three operations.
const (
	apiCommand = "api"
	methodFlag = "--method"
	methodPost = "POST"
	methodPut  = "PUT"
)

// Runs one schedule request, wrapping the runner's error so the cause travels
// with a note of what was attempted.
func (a *Applier) runSchedule(ctx context.Context, args ...string) error {
	if _, err := a.writer.Run(ctx, args...); err != nil {
		return fmt.Errorf("glab: %w", err)
	}
	return nil
}

// Signals a create whose response carried no id to address the new schedule by.
var errCreatedScheduleHasNoID = errors.New("created pipeline schedule has no id")

// Creates a schedule and then adds its variables, which need the id GitLab
// assigns and so cannot travel with the create.
func (a *Applier) createSchedule(ctx context.Context, endpoint string, desired manifest.PipelineSchedule) error {
	args := append([]string{apiCommand, endpoint, methodFlag, methodPost}, scheduleForm(desired)...)
	out, err := a.writer.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("glab: %w", err)
	}
	if len(desired.Variables) == 0 {
		return nil
	}
	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(out, &created); err != nil {
		return fmt.Errorf("parse created schedule: %w", err)
	}
	// Unmarshal accepts any object, so a response without an id leaves it zero
	// and the variables would be posted to pipeline_schedules/0. The schedule
	// itself exists by now, so the report has to say the variables are what is
	// missing rather than name a schedule nobody can look up.
	if created.ID == 0 {
		return fmt.Errorf("%w: %q was created without one, so its variables were not written",
			errCreatedScheduleHasNoID, desired.Description)
	}
	return a.applyVariables(ctx, fmt.Sprintf("%s/%d", endpoint, created.ID),
		planVariables(desired.Variables, nil))
}

// Reconciles one schedule's variables. GitLab keeps them behind endpoints of
// their own, addressed by key, so each addition, change and removal is its own
// request.
func (a *Applier) applyVariables(ctx context.Context, scheduleEndpoint string, plan variablePlan) error {
	endpoint := scheduleEndpoint + "/variables"
	for _, variable := range plan.added {
		if err := a.runSchedule(ctx, append([]string{apiCommand, endpoint, methodFlag, methodPost},
			variableCreateForm(variable)...)...); err != nil {
			return err
		}
	}
	for _, variable := range plan.changed {
		if err := a.runSchedule(ctx, append([]string{
			apiCommand, endpoint + "/" + glab.EncodePath(variable.Key), methodFlag, methodPut,
		}, variableUpdateForm(variable)...)...); err != nil {
			return err
		}
	}
	for _, key := range plan.removed {
		if err := a.runSchedule(ctx,
			apiCommand, endpoint+"/"+glab.EncodePath(key), methodFlag, "DELETE"); err != nil {
			return err
		}
	}
	return nil
}

// The form fields creating a variable. The key travels in the body here because
// a create has no path to put it in.
func variableCreateForm(variable manifest.ScheduleVariable) []string {
	return append([]string{"-f", "key=" + variable.Key}, variableUpdateForm(variable)...)
}

// The form fields an update sends. The key is absent because the path already
// carries it; spelling the fields out rather than slicing the create form keeps
// a field added to one from silently changing the other.
func variableUpdateForm(variable manifest.ScheduleVariable) []string {
	return []string{
		"-f", "value=" + variable.Value,
		"-f", "variable_type=" + string(variable.VariableType),
	}
}

// Updates one schedule. Its own attributes are written only when they differ:
// the variables travel through their own endpoints, so a schedule whose
// attributes already match needs no request of its own.
func (a *Applier) updateSchedule(
	ctx context.Context, endpoint string, desired manifest.PipelineSchedule, live LiveSchedule,
) error {
	scheduleEndpoint := fmt.Sprintf("%s/%d", endpoint, live.ID)
	if !sameScheduleAttributes(desired, live) {
		if err := a.runSchedule(ctx, append([]string{
			apiCommand, scheduleEndpoint, methodFlag, methodPut,
		}, scheduleForm(desired)...)...); err != nil {
			return err
		}
	}
	return a.applyVariables(ctx, scheduleEndpoint, planVariables(desired.Variables, live.Variables))
}
