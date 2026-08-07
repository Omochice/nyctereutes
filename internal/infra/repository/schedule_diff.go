package repository

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The manifest key every schedule change is reported under, which is also how
// [Applier] tells a schedule change from a project setting.
const fieldPipelineSchedules = "pipeline_schedules"

// The pipeline schedule one [Change] acts on. Which side is filled follows the
// change type: a create has no live schedule and a delete no declared one, so
// the description is held here rather than read off whichever side is present.
type ScheduleChange struct {
	// What the manifest calls the schedule, which is what pairs a declared
	// schedule with a live one.
	Description string
	// How GitLab addresses the schedule, zero for one that does not exist yet.
	ID int
	// What the manifest declares, zero for a deletion.
	Desired manifest.PipelineSchedule
	// What GitLab holds, zero for a creation.
	Live manifest.PipelineSchedule
}

// Reports how a project's live schedules differ from the declared ones, pairing
// them by description. A declared schedule with no live counterpart is created,
// one whose other attributes differ is updated, and a live schedule no
// declaration names is deleted.
//
// Deletions come first because [Applier.Apply] performs changes in the order it
// is given them, and an instance caps how many schedules a project may hold.
// Creating first would have that cap refuse the replacement on a project already
// at it, for a plan that only swaps one schedule for another.
func diffSchedules(name string, desired []manifest.PipelineSchedule, live []LiveSchedule) []Change {
	declared := make(map[string]struct{}, len(desired))
	for _, want := range desired {
		declared[want.Description] = struct{}{}
	}

	changes := make([]Change, 0, len(desired)+len(live))
	byDescription := make(map[string]LiveSchedule, len(live))
	for _, have := range live {
		byDescription[have.Description] = have
		if _, kept := declared[have.Description]; !kept {
			changes = append(changes, scheduleChange(ChangeDelete, name, ScheduleChange{
				Description: have.Description,
				ID:          have.ID,
				Live:        toManifestSchedule(have),
			}))
		}
	}

	for _, want := range desired {
		have, exists := byDescription[want.Description]
		switch {
		case !exists:
			changes = append(changes, scheduleChange(ChangeCreate, name, ScheduleChange{
				Description: want.Description,
				Desired:     want,
			}))
		case toManifestSchedule(have) != want:
			changes = append(changes, scheduleChange(ChangeUpdate, name, ScheduleChange{
				Description: want.Description,
				ID:          have.ID,
				Desired:     want,
				Live:        toManifestSchedule(have),
			}))
		}
	}
	return changes
}

// Wraps one schedule as a [Change], so the three producers cannot disagree
// about the field a schedule is reported under.
func scheduleChange(kind ChangeType, name string, schedule ScheduleChange) Change {
	return Change{Type: kind, Name: name, Field: fieldPipelineSchedules, Schedule: &schedule}
}

// The remaining schedule attribute names, kept as constants beside the ones
// [rejectIncompleteSchedules] uses so the scattered copies cannot drift apart.
const (
	fieldCronTimezone = "cron_timezone"
	fieldActive       = "active"
)

// One schedule attribute as a plan line reports it.
type scheduleAttribute struct {
	field string
	value string
}

// Renders a schedule's attributes in a fixed order, so two of them line up
// position by position. The description is absent because it is the identity: a
// schedule described differently is a different schedule rather than a changed
// one.
func scheduleAttributes(schedule manifest.PipelineSchedule) []scheduleAttribute {
	return []scheduleAttribute{
		{field: fieldRef, value: string(schedule.Ref)},
		{field: fieldCron, value: schedule.Cron},
		{field: fieldCronTimezone, value: schedule.CronTimezone},
		{field: fieldActive, value: strconv.FormatBool(schedule.Active)},
	}
}

// Renders the change as the lines a plan shows for it: a header naming the
// schedule, then the attributes the change is about. A create lists them all
// because none of them exists yet, an update lists only those that differ, and
// a delete lists none because the schedule goes whole.
func (s *ScheduleChange) line(kind ChangeType) string {
	header := fmt.Sprintf("pipeline_schedule %q", s.Description)
	switch kind {
	case ChangeCreate:
		declared := scheduleAttributes(s.Desired)
		lines := make([]string, 0, len(declared)+1)
		lines = append(lines, "+ "+header)
		for _, want := range declared {
			lines = append(lines, fmt.Sprintf("    + %s: %s", want.field, want.value))
		}
		return strings.Join(lines, "\n")
	case ChangeUpdate:
		live := scheduleAttributes(s.Live)
		lines := make([]string, 0, len(live)+1)
		lines = append(lines, "~ "+header)
		for index, want := range scheduleAttributes(s.Desired) {
			if live[index].value != want.value {
				lines = append(lines, fmt.Sprintf("    ~ %s: %s → %s", want.field, live[index].value, want.value))
			}
		}
		return strings.Join(lines, "\n")
	case ChangeDelete:
		return "- " + header
	default:
		return ""
	}
}
