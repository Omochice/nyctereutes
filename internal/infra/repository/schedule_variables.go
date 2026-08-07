package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// One schedule as the single-schedule endpoint reports it, read for its
// variables alone. The field is a pointer so its absence stays distinct from an
// empty list, which is the difference between a set nobody may see and a set
// that holds nothing.
type scheduleDetail struct {
	Variables *[]struct {
		Key string `json:"key"`
	} `json:"variables"`
}

// Signals a schedule whose variables the response did not carry. GitLab answers
// 200 with the field left out for a reader who is neither Maintainer, Owner,
// nor the schedule's creator, so an unreadable set arrives as a successful read
// rather than a refusal.
var errVariablesNotDisclosed = errors.New("pipeline schedule variables were not disclosed")

// The keys of one schedule's variables, for a plan about to destroy them.
// Values are not returned: nothing downstream may render or write one.
//
// The list endpoint carries no variables, so this addresses one schedule at a
// time and is worth making only where a change would destroy them.
func (c *Client) ScheduleVariableKeys(
	ctx context.Context, owner, name string, scheduleID int,
) ([]string, error) {
	endpoint := "projects/" + glab.EncodePath(owner+"/"+name) +
		"/pipeline_schedules/" + strconv.Itoa(scheduleID)
	out, err := c.runner.Run(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("read pipeline schedule %d of %s/%s: %w", scheduleID, owner, name, err)
	}
	var detail scheduleDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, fmt.Errorf("parse pipeline schedule %d of %s/%s: %w", scheduleID, owner, name, err)
	}
	if detail.Variables == nil {
		return nil, fmt.Errorf("%w: schedule %d of %s/%s", errVariablesNotDisclosed, scheduleID, owner, name)
	}
	keys := make([]string, 0, len(*detail.Variables))
	for _, variable := range *detail.Variables {
		keys = append(keys, variable.Key)
	}
	return keys, nil
}
