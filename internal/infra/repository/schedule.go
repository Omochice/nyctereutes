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
}

// Reads every pipeline schedule a project owns. The endpoint pages at 20 while
// an instance can raise the schedule limit above that, so glab is asked to
// follow the pages rather than the project being described by its first page.
//
// A schedule's variables are not read. They are not part of the manifest, and
// the only endpoint carrying them answers one schedule at a time.
func (c *Client) FetchSchedules(ctx context.Context, owner, name string) ([]LiveSchedule, error) {
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
	return schedules, nil
}

// The attributes a schedule must carry to be describable or addressable. They
// repeat the JSON tags because a tag cannot name a constant.
const (
	fieldID   = "id"
	fieldRef  = "ref"
	fieldCron = "cron"
)

// Signals a schedule GitLab reports without an attribute a manifest requires.
var errIncompleteLiveSchedule = errors.New("incomplete pipeline schedule")

// Rejects a schedule reported without one of the three the schema requires of a
// declared one, so neither side trusts the other to have looked. The schema's
// own check runs on parse, which a document built from live state never goes
// through. GitLab skips its presence checks for a schedule brought in by its
// project import, so a project can hold one that cannot be described.
//
// The id is required too, though no manifest holds it: one reported without it
// would have its update or delete addressed to pipeline_schedules/0.
func rejectIncompleteSchedules(schedules []LiveSchedule) error {
	for _, schedule := range schedules {
		if schedule.ID == 0 {
			return fmt.Errorf("%w: schedule %q reports no %s",
				errIncompleteLiveSchedule, schedule.Description, fieldID)
		}
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
		schedules = append(schedules, toManifestSchedule(schedule))
	}
	slices.SortFunc(schedules, func(left, right manifest.PipelineSchedule) int {
		return cmp.Compare(left.Description, right.Description)
	})
	return schedules
}

// Describes one live schedule the way a manifest would, which is what makes it
// comparable with a declared one.
func toManifestSchedule(live LiveSchedule) manifest.PipelineSchedule {
	return manifest.PipelineSchedule{
		Description:  live.Description,
		Ref:          manifest.Ref(live.Ref),
		Cron:         live.Cron,
		CronTimezone: live.CronTimezone,
		Active:       live.Active,
	}
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
