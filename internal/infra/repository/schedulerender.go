package repository

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Renders a whole-schedule change: a marked header naming the schedule, then
// the attributes indented beneath it. A create lists every attribute because
// nothing exists to compare against; an update lists only the ones that differ,
// so a plan does not restate what already matches. A delete has nothing to show
// beyond which schedule goes.
func renderScheduleChange(change Change) string {
	switch change.Type {
	case ChangeDelete:
		live, _ := change.OldValue.(LiveSchedule)
		// The variables go with the schedule, so any that were read are named
		// here: nothing else in the plan mentions them. A schedule whose
		// variables went unread, which is every schedule when no manifest
		// manages any, is rendered without them. Values never appear; they exist
		// only on GitLab and a plan is read aloud and pasted into issues.
		return scheduleBlock("-", live.Description, removedVariables(live.Variables))
	case ChangeCreate:
		desired, _ := change.NewValue.(manifest.PipelineSchedule)
		return scheduleBlock("+", desired.Description,
			append(createdAttributes(desired), variableAttributes(desired, nil)...))
	case ChangeUpdate:
		desired, _ := change.NewValue.(manifest.PipelineSchedule)
		live, _ := change.OldValue.(LiveSchedule)
		return scheduleBlock("~", desired.Description,
			append(changedAttributes(desired, live), variableAttributes(desired, live.Variables)...))
	default:
		return ""
	}
}

// One line of a schedule block. A named attribute is padded to a common width
// with the others; a line with no name is written verbatim, which is how the
// variable entries keep their own markers.
type scheduleAttribute struct {
	name  string
	value string
}

// Every attribute a create writes, in the order the manifest declares them.
func createdAttributes(desired manifest.PipelineSchedule) []scheduleAttribute {
	return []scheduleAttribute{
		{name: fieldRef, value: string(desired.Ref)},
		{name: fieldCron, value: desired.Cron},
		{name: fieldCronTimezone, value: desired.CronTimezone},
		{name: fieldActive, value: strconv.FormatBool(desired.Active)},
	}
}

// The attributes whose live value differs from the declared one, each shown as
// the transition it describes.
func changedAttributes(desired manifest.PipelineSchedule, live LiveSchedule) []scheduleAttribute {
	var changed []scheduleAttribute
	for _, attribute := range []struct{ name, was, becomes string }{
		{name: fieldRef, was: live.Ref, becomes: string(desired.Ref)},
		{name: fieldCron, was: live.Cron, becomes: desired.Cron},
		{name: fieldCronTimezone, was: live.CronTimezone, becomes: desired.CronTimezone},
		{
			name:    fieldActive,
			was:     strconv.FormatBool(live.Active),
			becomes: strconv.FormatBool(desired.Active),
		},
	} {
		if attribute.was != attribute.becomes {
			changed = append(changed, scheduleAttribute{
				name:  attribute.name,
				value: fmt.Sprintf("%s → %s", attribute.was, attribute.becomes),
			})
		}
	}
	return changed
}

// Assembles the header and its attribute lines, padding the names to a common
// width so the values line up under one another.
func scheduleBlock(marker, description string, attributes []scheduleAttribute) string {
	if len(attributes) == 0 {
		return fmt.Sprintf("%s pipeline_schedule %q", marker, description)
	}
	lines := make([]string, 0, 1+len(attributes))
	lines = append(lines, fmt.Sprintf("%s pipeline_schedule %q:", marker, description))
	width := 0
	for _, attribute := range attributes {
		if attribute.name != "" {
			width = max(width, len(attribute.name))
		}
	}
	for _, attribute := range attributes {
		if attribute.name == "" {
			lines = append(lines, "    "+attribute.value)
			continue
		}
		lines = append(lines, fmt.Sprintf("    %-*s %s", width+1, attribute.name+":", attribute.value))
	}
	return strings.Join(lines, "\n")
}

// The header a block's variable lines sit under.
const variablesHeading = "variables:"

// The variable lines of a deleted schedule. Every one of them goes with it, and
// a plan that named only the schedule would let their values disappear without
// the reader ever seeing what they were called.
func removedVariables(live []ScheduleVariable) []scheduleAttribute {
	if len(live) == 0 {
		return nil
	}
	keys := make([]string, 0, len(live))
	for _, variable := range live {
		keys = append(keys, variable.Key)
	}
	slices.Sort(keys)
	lines := []scheduleAttribute{{value: variablesHeading}}
	for _, key := range keys {
		lines = append(lines, scheduleAttribute{value: "  - " + key})
	}
	return lines
}

// The variable lines of a block. Only the keys and what happens to each are
// shown, because the manifest beside the plan already states every value and
// repeating them says nothing new. This is not a redaction: import writes the
// values to stdout by design, since a document without them cannot be applied.
func variableAttributes(desired manifest.PipelineSchedule, live []ScheduleVariable) []scheduleAttribute {
	plan := planVariables(desired.Variables, live)
	if plan.empty() {
		return nil
	}
	lines := []scheduleAttribute{{value: variablesHeading}}
	for _, variable := range plan.added {
		lines = append(lines, scheduleAttribute{value: "  + " + variable.Key})
	}
	for _, variable := range plan.changed {
		lines = append(lines, scheduleAttribute{value: "  ~ " + variable.Key})
	}
	for _, key := range plan.removed {
		lines = append(lines, scheduleAttribute{value: "  - " + key})
	}
	return lines
}
