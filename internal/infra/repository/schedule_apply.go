package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Creates, updates or removes one pipeline schedule. GitLab addresses it by an
// id the manifest never holds, so an update and a delete carry the one the read
// reported while a create has none.
func (a *Applier) applySchedule(ctx context.Context, change Change) error {
	schedule := change.Schedule
	if schedule == nil {
		return fmt.Errorf("%w: %s carries no schedule", errUnexpectedValueType, change.Field)
	}
	collection := "projects/" + glab.EncodePath(change.Name) + "/pipeline_schedules"
	field := scheduleField(schedule.Description)
	switch change.Type {
	case ChangeCreate:
		args := append([]string{"api", collection, "--method", "POST"}, scheduleForm(schedule.Desired)...)
		_, err := a.writer.Run(ctx, args...)
		return wrapWrite(err, change.Name, field)
	case ChangeUpdate:
		one := collection + "/" + strconv.Itoa(schedule.ID)
		args := append([]string{"api", one, "--method", "PUT"}, scheduleForm(schedule.Desired)...)
		_, err := a.writer.Run(ctx, args...)
		return wrapWrite(err, change.Name, field)
	case ChangeDelete:
		one := collection + "/" + strconv.Itoa(schedule.ID)
		_, err := a.writer.Run(ctx, "api", one, "--method", "DELETE")
		return wrapWrite(err, change.Name, field)
	default:
		return fmt.Errorf("%w: %s is not a schedule change", errUnexpectedValueType, change.Type)
	}
}

// Names the schedule as the field a failure is reported under, in the form the
// plan line uses.
func scheduleField(description string) string {
	return fmt.Sprintf("%s %q", schedulePrefix, description)
}

// The form fields a create or an update sends. Every attribute goes on both,
// because GitLab's update changes only the ones it is given, and an omitted one
// would leave the schedule on the state the plan said would be replaced.
func scheduleForm(schedule manifest.PipelineSchedule) []string {
	return []string{
		"-f", fieldDescription + "=" + schedule.Description,
		"-f", fieldRef + "=" + string(schedule.Ref),
		"-f", fieldCron + "=" + schedule.Cron,
		"-f", fieldCronTimezone + "=" + schedule.CronTimezone,
		"-f", fieldActive + "=" + strconv.FormatBool(schedule.Active),
	}
}
