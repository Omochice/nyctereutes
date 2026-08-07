package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// One schedule as the single-schedule endpoint reports it, read for its
// variables alone. The pointer keeps an absent field distinct from an empty
// list: a set nobody may see against one that holds nothing.
type scheduleDetail struct {
	Variables *[]struct {
		Key string `json:"key"`
	} `json:"variables"`
}

// Signals a schedule whose variables the response did not carry. GitLab leaves
// the field out of a 200 for a reader who is neither Maintainer, Owner, nor the
// schedule's creator, so an unreadable set arrives as a successful read.
var errVariablesNotDisclosed = errors.New("pipeline schedule variables were not disclosed")

// The keys of one schedule's variables, for a plan about to destroy them.
// Values are not returned: nothing downstream may render or write one.
//
// The list endpoint carries no variables, so this is one request per schedule
// and worth making only where a change would destroy them.
//
// The keys come back sorted. GitLab documents no order for them, and a plan an
// operator reads twice should not shuffle the lines under a removal.
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
	slices.Sort(keys)
	return keys, nil
}
