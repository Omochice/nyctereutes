package repository

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// One pipeline schedule as GitLab reports it. The id never reaches a manifest,
// which identifies a schedule by description, but it is the only way the API
// addresses one for an update or a delete, so the live state carries it.
type LiveSchedule struct {
	ID           int    `json:"id"`
	Description  string `json:"description"`
	Ref          string `json:"ref"`
	Cron         string `json:"cron"`
	CronTimezone string `json:"cron_timezone"`
	Active       bool   `json:"active"`
	// Read from the single-schedule endpoint, which is the only place GitLab
	// reports them, and only when a caller asks: the read costs one request per
	// schedule. Nil therefore means "not read", which is why a manifest that
	// declares no variable never triggers the read.
	Variables []ScheduleVariable `json:"variables"`
}

// One variable attached to a pipeline schedule. The value is free text the same
// way a description or a template is, and a file-typed one is routinely
// multiline, so it carries the type that normalizes line endings: a bare CR
// reaching the manifest would cost Marshal its literal block.
type ScheduleVariable struct {
	Key          string   `json:"key"`
	Value        freeText `json:"value"`
	VariableType string   `json:"variable_type"`
}

// Reads every pipeline schedule a project owns. The endpoint pages at 20 while
// an instance can raise the schedule limit above that, so glab is asked to
// follow the pages rather than the project being described by its first page.
//
// The variables cost one request per schedule and are read only when
// withVariables asks for them: a caller that manages no variable cannot use
// them, and paying for them regardless turns one project read into as many
// requests as the project has schedules.
//
// A schedule whose variables the token may not see keeps them nil even when
// withVariables asked, so having asked is what makes nil mean "not permitted"
// rather than "not requested". [SchedulesMissingVariables] names those for a
// caller that wants to say so.
func (c *Client) FetchSchedules(
	ctx context.Context, owner, name string, withVariables bool,
) ([]LiveSchedule, error) {
	out, err := c.runner.Run(ctx, "api", "--paginate",
		"projects/"+glab.EncodePath(owner+"/"+name)+"/pipeline_schedules")
	if err != nil {
		return nil, fmt.Errorf("fetch pipeline schedules %s/%s: %w", owner, name, err)
	}
	schedules, err := decodeSchedulePages(out)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline schedules %s/%s: %w", owner, name, err)
	}
	if err := rejectIncompleteSchedules(schedules); err != nil {
		return nil, fmt.Errorf("read pipeline schedules %s/%s: %w", owner, name, err)
	}
	if err := rejectDuplicateDescriptions(schedules); err != nil {
		return nil, fmt.Errorf("read pipeline schedules %s/%s: %w", owner, name, err)
	}
	if !withVariables {
		return schedules, nil
	}
	for index := range schedules {
		variables, err := c.fetchScheduleVariables(ctx, owner, name, schedules[index].ID)
		if err != nil {
			return nil, err
		}
		schedules[index].Variables = variables
	}
	return schedules, nil
}

// Signals a schedule GitLab reports without an attribute a manifest requires.
var errIncompleteLiveSchedule = errors.New("incomplete pipeline schedule")

// Rejects a schedule reported without one of the three the schema requires of a
// declared one, so neither side trusts the other to have looked. The schema's
// own check runs on parse, which a document built from live state never goes
// through. GitLab skips its presence checks for a schedule brought in by its
// project import, so a project can hold one that cannot be described.
func rejectIncompleteSchedules(schedules []LiveSchedule) error {
	for _, schedule := range schedules {
		for _, required := range []struct{ field, value string }{
			{field: fieldDescription, value: schedule.Description},
			{field: fieldRef, value: schedule.Ref},
			{field: fieldCron, value: schedule.Cron},
		} {
			if required.value == "" {
				return fmt.Errorf("%w: schedule %d (%q) reports no %s",
					errIncompleteLiveSchedule, schedule.ID, schedule.Description, required.field)
			}
		}
	}
	return nil
}

// Reads one schedule's variables. The list response omits them entirely, so
// knowing whether a schedule carries any costs a request per schedule.
//
// GitLab answers a reader who is neither Maintainer, Owner, nor the schedule's
// creator with the field left out rather than with an error, so nil is returned
// for it and the schedule stays undescribed on that one attribute. A pointer is
// what tells that omission apart from the empty list a permitted reader gets
// for a schedule carrying none, which is a description rather than a silence.
func (c *Client) fetchScheduleVariables(
	ctx context.Context, owner, name string, scheduleID int,
) ([]ScheduleVariable, error) {
	endpoint := fmt.Sprintf("projects/%s/pipeline_schedules/%d", glab.EncodePath(owner+"/"+name), scheduleID)
	out, err := c.runner.Run(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch pipeline schedule %d on %s/%s: %w", scheduleID, owner, name, err)
	}
	var schedule struct {
		Variables *[]ScheduleVariable `json:"variables"`
	}
	if err := json.Unmarshal(out, &schedule); err != nil {
		return nil, fmt.Errorf("parse pipeline schedule %d on %s/%s: %w", scheduleID, owner, name, err)
	}
	if schedule.Variables == nil {
		return nil, nil
	}
	return *schedule.Variables, nil
}

// Names the schedules whose variables were asked for and not answered, which is
// how GitLab replies to a reader below Maintainer who does not own them. Only a
// caller that passed withVariables may read the result: without that, every
// schedule is missing its variables and the answer means nothing.
func SchedulesMissingVariables(schedules []LiveSchedule) []string {
	var missing []string
	for _, schedule := range schedules {
		if schedule.Variables == nil {
			missing = append(missing, schedule.Description)
		}
	}
	return missing
}

// Signals a project carrying two schedules described alike.
var errDuplicateLiveSchedule = errors.New("duplicate pipeline schedule description")

// Rejects a project whose schedules repeat a description. GitLab allows it, but
// a manifest pairs a declared schedule with a live one by description, so such
// a project cannot be described: an update or a delete would land on an
// arbitrary member of the pair. Both ids are named because renaming one of them
// is what resolves it.
func rejectDuplicateDescriptions(schedules []LiveSchedule) error {
	seen := make(map[string]int, len(schedules))
	for _, schedule := range schedules {
		if first, repeated := seen[schedule.Description]; repeated {
			return fmt.Errorf("%w: %q is used by schedules %d and %d",
				errDuplicateLiveSchedule, schedule.Description, first, schedule.ID)
		}
		seen[schedule.Description] = schedule.ID
	}
	return nil
}

// Converts the live schedules into manifest documents, ordered by description.
// GitLab lists them in id order, which is creation order, so a schedule deleted
// and recreated would move; a manifest is a file under version control, so the
// same live state has to produce the same bytes. The description is already
// unique across a document, which makes the order total.
//
// Schedules that were never read pass through as nil, which the field on
// [manifest.RepositorySpec] keeps distinct from the empty list.
func toManifestSchedules(live []LiveSchedule) []manifest.PipelineSchedule {
	if live == nil {
		return nil
	}
	schedules := make([]manifest.PipelineSchedule, 0, len(live))
	for _, schedule := range live {
		schedules = append(schedules, manifest.PipelineSchedule{
			Description:  schedule.Description,
			Ref:          manifest.Ref(schedule.Ref),
			Cron:         schedule.Cron,
			CronTimezone: schedule.CronTimezone,
			Active:       schedule.Active,
			Variables:    toManifestVariables(schedule.Variables),
		})
	}
	slices.SortFunc(schedules, func(left, right manifest.PipelineSchedule) int {
		return cmp.Compare(left.Description, right.Description)
	})
	return schedules
}

// Signals output that carries no page of schedules to read.
var errNoSchedulePage = errors.New("response carries no pipeline schedule list")

// Joins the pages glab wrote. In --paginate mode it emits one JSON array per
// page back to back rather than a single merged array, so the whole output is
// not one JSON value and has to be decoded in sequence.
//
// Output holding no page, or a page written as null, is refused rather than
// read as a project owning no schedule, which the field on
// [manifest.RepositorySpec] would carry into a document as a declaration.
func decodeSchedulePages(out []byte) ([]LiveSchedule, error) {
	decoder := json.NewDecoder(bytes.NewReader(out))
	schedules := []LiveSchedule{}
	pages := 0
	for {
		var page []LiveSchedule
		if err := decoder.Decode(&page); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode page: %w", err)
		}
		if page == nil {
			return nil, errNoSchedulePage
		}
		schedules = append(schedules, page...)
		pages++
	}
	if pages == 0 {
		return nil, errNoSchedulePage
	}
	return schedules, nil
}

// Converts a schedule's variables, ordered by key for the reason the schedules
// themselves are ordered by description: GitLab guarantees no order, and the
// document has to be stable under version control.
//
// Variables that were never read pass through as nil, which the field on
// [manifest.PipelineSchedule] keeps distinct from the empty list.
func toManifestVariables(live []ScheduleVariable) []manifest.ScheduleVariable {
	if live == nil {
		return nil
	}
	variables := make([]manifest.ScheduleVariable, 0, len(live))
	for _, variable := range live {
		variables = append(variables, manifest.ScheduleVariable{
			Key:          variable.Key,
			Value:        string(variable.Value),
			VariableType: manifest.VariableType(variable.VariableType),
		})
	}
	slices.SortFunc(variables, func(left, right manifest.ScheduleVariable) int {
		return cmp.Compare(left.Key, right.Key)
	})
	return variables
}
