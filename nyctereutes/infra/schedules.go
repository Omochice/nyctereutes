package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

// Reads the project's schedules into state when the manifest manages them.
// A manifest that declares no pipeline_schedules block manages none, so the
// read is skipped: it lives behind endpoints that can fail or be ambiguous on
// their own terms, and folding it into every project read would let a schedule
// problem hide the drift of every other setting.
//
// The variables cost a request per schedule, so they are read only when the
// manifest declares the block on some schedule. An empty list counts: it
// declares that the schedule should carry none, which still has to be compared
// against what is live.
func loadSchedules(
	ctx context.Context, client *repository.Client, repo *manifest.Repository, state *repository.CurrentState,
) error {
	if state.IsNew || repo.Spec.PipelineSchedules == nil {
		return nil
	}
	withVariables := declaresVariables(repo.Spec.PipelineSchedules)
	schedules, err := client.FetchSchedules(
		ctx, repo.Metadata.Owner, repo.Metadata.Name, withVariables,
	)
	if err != nil {
		return fmt.Errorf("read schedules for %s/%s: %w", repo.Metadata.Owner, repo.Metadata.Name, err)
	}
	if withVariables {
		if err := refuseHiddenVariables(repo, schedules); err != nil {
			return err
		}
	}
	state.PipelineSchedules = schedules
	return nil
}

// Signals a project whose live variables the token may not read.
var errVariablesNotReadable = errors.New("pipeline schedule variables are not readable")

// Stops a run that would reconcile variables it cannot see. GitLab answers a
// reader who is neither Maintainer, Owner, nor the schedule's creator with the
// variables left out, and reading that absence as a schedule carrying none
// would plan every declared variable as an addition and every live one as
// already gone. The token cannot write them either, so there is nothing to
// gain by continuing.
func refuseHiddenVariables(repo *manifest.Repository, schedules []repository.LiveSchedule) error {
	hidden := repository.SchedulesMissingVariables(schedules)
	if len(hidden) == 0 {
		return nil
	}
	return fmt.Errorf("read schedules for %s/%s: %w: %s",
		repo.Metadata.Owner, repo.Metadata.Name, errVariablesNotReadable, strings.Join(hidden, ", "))
}

// Reports whether any declared schedule manages its variables.
func declaresVariables(schedules []manifest.PipelineSchedule) bool {
	for _, schedule := range schedules {
		if schedule.Variables != nil {
			return true
		}
	}
	return false
}
