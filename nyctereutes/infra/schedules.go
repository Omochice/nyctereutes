package infra

import (
	"context"
	"fmt"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

// Reads the project's schedules into state when the manifest manages them.
// A manifest that declares no pipeline_schedules block manages none, so the
// read is skipped: it lives behind endpoints that can fail or be ambiguous on
// their own terms, and folding it into every project read would let a schedule
// problem hide the drift of every other setting.
//
// The variables cost a request per schedule, so they are read only when a
// declared schedule names any. Nothing can consume them otherwise.
func loadSchedules(
	ctx context.Context, client *repository.Client, repo *manifest.Repository, state *repository.CurrentState,
) error {
	if state.IsNew || repo.Spec.PipelineSchedules == nil {
		return nil
	}
	schedules, err := client.FetchSchedules(
		ctx, repo.Metadata.Owner, repo.Metadata.Name, declaresVariables(repo.Spec.PipelineSchedules),
	)
	if err != nil {
		return fmt.Errorf("read schedules for %s/%s: %w", repo.Metadata.Owner, repo.Metadata.Name, err)
	}
	state.PipelineSchedules = schedules
	return nil
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
