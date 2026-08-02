package repository

import (
	"cmp"
	"slices"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The plan field naming a whole-schedule change. One change carries one
// schedule rather than one attribute: GitLab addresses a schedule by an id the
// manifest does not hold, so apply reads it from the change's live value, and
// a create needs every attribute anyway.
const fieldPipelineSchedule = "pipeline_schedule"

// Reports how the live schedules differ from the declared ones, pairing them by
// description. A nil declaration manages no schedule and yields nothing; a
// declared list is the complete desired set, so a live schedule it does not
// name is removed.
//
// Deletes are emitted before creates because a project's schedule count is
// capped, and a plan that replaces a schedule while the project sits at the
// limit fails if the create goes first.
func diffSchedules(changes *[]Change, name string, desired []manifest.PipelineSchedule, live []LiveSchedule) {
	if desired == nil {
		return
	}
	declared := make(map[string]manifest.PipelineSchedule, len(desired))
	for _, schedule := range desired {
		declared[schedule.Description] = schedule
	}
	current := make(map[string]LiveSchedule, len(live))
	for _, schedule := range live {
		current[schedule.Description] = schedule
	}

	for _, schedule := range sortedByDescription(live) {
		if _, kept := declared[schedule.Description]; !kept {
			*changes = append(*changes, Change{
				Type: ChangeDelete, Name: name, Field: fieldPipelineSchedule,
				OldValue: schedule,
			})
		}
	}
	for _, schedule := range desired {
		existing, found := current[schedule.Description]
		switch {
		case !found:
			*changes = append(*changes, Change{
				Type: ChangeCreate, Name: name, Field: fieldPipelineSchedule,
				NewValue: schedule,
			})
		case !sameSchedule(schedule, existing):
			*changes = append(*changes, Change{
				Type: ChangeUpdate, Name: name, Field: fieldPipelineSchedule,
				OldValue: existing, NewValue: schedule,
			})
		}
	}
}

// Orders live schedules by description so the emitted deletes do not depend on
// the id order GitLab happens to list them in.
func sortedByDescription(live []LiveSchedule) []LiveSchedule {
	ordered := slices.Clone(live)
	slices.SortFunc(ordered, func(left, right LiveSchedule) int {
		return cmp.Compare(left.Description, right.Description)
	})
	return ordered
}

// Reports whether the live schedule already matches the declared one. Every
// attribute the manifest carries is compared; the ones it does not hold, such
// as the id and the owner, are GitLab's to decide.
func sameSchedule(desired manifest.PipelineSchedule, live LiveSchedule) bool {
	return sameScheduleAttributes(desired, live) &&
		planVariables(desired.Variables, live.Variables).empty()
}

// Reports whether the schedule's own attributes match, leaving its variables
// aside. They are written through different endpoints, so apply asks about them
// separately.
func sameScheduleAttributes(desired manifest.PipelineSchedule, live LiveSchedule) bool {
	return string(desired.Ref) == live.Ref &&
		desired.Cron == live.Cron &&
		desired.CronTimezone == live.CronTimezone &&
		desired.Active == live.Active
}
