package repository

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The field every schedule change is reported under, which is how [Applier]
// tells one from a project setting.
const fieldPipelineSchedules = "pipeline_schedules"

// The pipeline schedule one [Change] acts on. Only the side the change type
// implies is filled: a creation has no Live and an id of zero, a deletion no
// Desired. Description is held separately because it is the one thing every
// change carries, and it is what pairs a declared schedule with a live one.
type ScheduleChange struct {
	Description string
	// How GitLab addresses the schedule.
	ID      int
	Desired manifest.PipelineSchedule
	Live    manifest.PipelineSchedule
	// The keys of the variables a removal would destroy, filled by the command
	// that plans the removal and empty everywhere else. The manifest holds no
	// variables, so nothing else in a plan would mention them.
	VariableKeys []string
}

// Reports how a project's live schedules differ from the declared ones, pairing
// them by description.
//
// Deletions come first because [Applier.Apply] performs changes in the order it
// is given them, and an instance caps how many schedules a project may hold, so
// creating first would have that cap refuse a replacement on a project at it.
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
// about the field it is reported under.
func scheduleChange(kind ChangeType, name string, schedule ScheduleChange) Change {
	return Change{Type: kind, Name: name, Field: fieldPipelineSchedules, Schedule: &schedule}
}

// The schedule attribute names not already named for
// [rejectIncompleteSchedules].
const (
	fieldCronTimezone = "cron_timezone"
	fieldActive       = "active"
)

type scheduleAttribute struct {
	field string
	value string
}

// Renders a schedule's attributes in a fixed order, so two of them line up
// position by position. The description is absent because it is the identity: a
// schedule described differently is a different one, not a changed one.
func scheduleAttributes(schedule manifest.PipelineSchedule) []scheduleAttribute {
	return []scheduleAttribute{
		{field: fieldRef, value: string(schedule.Ref)},
		{field: fieldCron, value: schedule.Cron},
		{field: fieldCronTimezone, value: schedule.CronTimezone},
		{field: fieldActive, value: strconv.FormatBool(schedule.Active)},
	}
}

// Renders the change as the lines a plan shows for it. A creation lists every
// attribute because none of them exists yet, an update only those that differ,
// and a deletion none, naming instead the variable keys going with the schedule.
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
		lines := make([]string, 0, len(s.VariableKeys)+1)
		lines = append(lines, "- "+header)
		for _, key := range s.VariableKeys {
			lines = append(lines, "    - variable: "+key)
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}
