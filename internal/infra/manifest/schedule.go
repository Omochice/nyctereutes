package manifest

import (
	"fmt"
	"strings"
)

// One recurring pipeline run a project owns. Unlike the project settings in
// [RepositorySpec], a schedule is a child resource GitLab addresses by a
// server-assigned id; the manifest identifies it by description instead, so no
// server-assigned value enters the document.
type PipelineSchedule struct {
	Description  string `yaml:"description"`
	Ref          Ref    `yaml:"ref"`
	Cron         string `yaml:"cron"`
	CronTimezone string `yaml:"cron_timezone"`
	Active       bool   `yaml:"active"`
}

// What GitLab stores when a schedule is created without a timezone.
const defaultCronTimezone = "UTC"

// The attribute names reported by the required-field check. They repeat the
// struct tags because a tag cannot name a constant.
const (
	fieldDescription = "description"
	fieldRef         = "ref"
	fieldCron        = "cron"
)

// Decodes a schedule, filling the attributes GitLab defaults on create. The
// defaults are seeded before decoding so a declared value overwrites them,
// which is what keeps a schedule paused in the manifest from coming back on.
//
// Decoding runs through the callback rather than from a byte slice, so the
// surrounding decoder does the work: its strictness applies inside the schedule
// too, and its errors point at the line in the file rather than at an offset
// counted from the schedule's own start.
func (schedule *PipelineSchedule) UnmarshalYAML(unmarshal func(any) error) error {
	// The local type drops this method, so decoding into it does not recurse.
	type scheduleFields PipelineSchedule
	fields := scheduleFields{CronTimezone: defaultCronTimezone, Active: true}
	if err := unmarshal(&fields); err != nil {
		return fmt.Errorf("decode pipeline schedule: %w", err)
	}
	*schedule = PipelineSchedule(fields)
	return nil
}

// Reports the first required attribute the schedule leaves empty, locating it
// by its position in the document. All three are required to create a schedule
// through the API, so one missing any of them could never apply; checking here
// turns a remote rejection into a local error.
func (schedule *PipelineSchedule) validate(index int) error {
	for _, required := range []struct{ field, value string }{
		{field: fieldDescription, value: schedule.Description},
		{field: fieldRef, value: string(schedule.Ref)},
		{field: fieldCron, value: schedule.Cron},
	} {
		if required.value == "" {
			return fmt.Errorf("%w: spec.pipeline_schedules[%d].%s", errRequiredField, index, required.field)
		}
	}
	return nil
}

// The git ref a schedule runs against, always held as the full path GitLab
// reports ("refs/heads/main", "refs/tags/v1.0.0"). Shortening it to the name the
// web UI displays would leave a branch and a tag of the same name
// indistinguishable.
type Ref string

// The prefix a bare ref name is taken to carry.
const branchRefPrefix = "refs/heads/"

// Expands a bare name to a branch ref while decoding, so the short form a
// person writes and the full form GitLab reports compare equal. Anything
// already under refs/ is a full path and is kept as written.
//
// Decoding goes through the callback for the reason [PipelineSchedule] does: a
// detached byte slice carries no position, so a ref of the wrong shape would
// report a line counted from the value itself.
func (ref *Ref) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return fmt.Errorf("decode ref: %w", err)
	}
	*ref = canonicalRef(Ref(value))
	return nil
}

// Expands a bare name to a branch ref, which is what decoding does. Marshal
// applies it too, because a document built in memory never decodes; a bare ref
// that only decoding expanded would fail the round-trip check with an error
// naming no field.
func canonicalRef(ref Ref) Ref {
	if ref == "" || strings.HasPrefix(string(ref), "refs/") {
		return ref
	}
	return branchRefPrefix + ref
}
