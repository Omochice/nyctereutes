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
//
// Only a schedule the manifest manages the variables of is checked. The read
// asks for every schedule's variables because the endpoint is per schedule and
// the manifest pairs by description, so a project can carry schedules nobody
// declared; those going unread costs the plan nothing.
func refuseHiddenVariables(repo *manifest.Repository, schedules []repository.LiveSchedule) error {
	managed := managedVariableDescriptions(repo.Spec.PipelineSchedules)
	var hidden []string
	for _, description := range repository.SchedulesMissingVariables(schedules) {
		if _, ok := managed[description]; ok {
			hidden = append(hidden, description)
		}
	}
	if len(hidden) == 0 {
		return nil
	}
	return fmt.Errorf("read schedules for %s/%s: %w: %s",
		repo.Metadata.Owner, repo.Metadata.Name, errVariablesNotReadable, strings.Join(hidden, ", "))
}

// The descriptions of the schedules whose variables the manifest declares.
func managedVariableDescriptions(schedules []manifest.PipelineSchedule) map[string]struct{} {
	managed := make(map[string]struct{}, len(schedules))
	for _, schedule := range schedules {
		if schedule.Variables != nil {
			managed[schedule.Description] = struct{}{}
		}
	}
	return managed
}

// Reports whether any declared schedule manages its variables.
func declaresVariables(schedules []manifest.PipelineSchedule) bool {
	return len(managedVariableDescriptions(schedules)) > 0
}
