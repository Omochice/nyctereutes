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
	if err := rejectDuplicateDescriptions(schedules); err != nil {
		return nil, fmt.Errorf("read pipeline schedules %s/%s: %w", owner, name, err)
	}
	return schedules, nil
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
func toManifestSchedules(live []LiveSchedule) []manifest.PipelineSchedule {
	schedules := make([]manifest.PipelineSchedule, 0, len(live))
	for _, schedule := range live {
		schedules = append(schedules, manifest.PipelineSchedule{
			Description:  schedule.Description,
			Ref:          manifest.CanonicalRef(manifest.Ref(schedule.Ref)),
			Cron:         schedule.Cron,
			CronTimezone: schedule.CronTimezone,
			Active:       schedule.Active,
		})
	}
	slices.SortFunc(schedules, func(left, right manifest.PipelineSchedule) int {
		return cmp.Compare(left.Description, right.Description)
	})
	return schedules
}

// Joins the pages glab wrote. In --paginate mode it emits one JSON array per
// page back to back rather than a single merged array, so the whole output is
// not one JSON value and has to be decoded in sequence.
func decodeSchedulePages(out []byte) ([]LiveSchedule, error) {
	decoder := json.NewDecoder(bytes.NewReader(out))
	var schedules []LiveSchedule
	for {
		var page []LiveSchedule
		if err := decoder.Decode(&page); err != nil {
			if errors.Is(err, io.EOF) {
				return schedules, nil
			}
			return nil, fmt.Errorf("decode page: %w", err)
		}
		schedules = append(schedules, page...)
	}
}
