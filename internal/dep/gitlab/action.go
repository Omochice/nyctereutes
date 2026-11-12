package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/dep/types"
	"github.com/Omochice/nyctereutes/internal/glab"
)

// Approves a merge request via glab.
//
// Approval is idempotent from the caller's perspective: GitLab answers a repeat
// approval from the same user with 401 Unauthorized, so an already-approved MR
// is reported as success instead of aborting a follow-up merge. A genuinely
// unauthorized token returns 401 too, but then the follow-up merge fails on its
// own, so treating it as success here is safe.
func (c *Client) ApproveMR(ctx context.Context, mergeRequest types.MR) error {
	_, err := c.runner.Run(ctx, "mr", "approve", strconv.Itoa(mergeRequest.IID), "-R", mergeRequest.Project)
	if err == nil || errors.Is(err, glab.ErrUnauthorized) {
		return nil
	}
	return fmt.Errorf("failed to approve MR !%d: %w", mergeRequest.IID, err)
}

// Merges a merge request via glab as options describe.
func (c *Client) MergeMR(ctx context.Context, mergeRequest types.MR, options types.MergeOptions) error {
	args := []string{"mr", "merge", strconv.Itoa(mergeRequest.IID), "-R", mergeRequest.Project, "--yes"}

	switch options.Method {
	case types.MergeMethodSquash:
		args = append(args, "--squash")
	case types.MergeMethodRebase:
		args = append(args, "--rebase")
	case types.MergeMethodMerge:
		// glab merges plainly when no method flag is given.
	}

	if options.AutoMerge {
		args = append(args, "--auto-merge")
	} else {
		args = append(args, "--auto-merge=false")
	}

	if _, err := c.runner.Run(ctx, args...); err != nil {
		return fmt.Errorf("failed to merge MR !%d: %w", mergeRequest.IID, err)
	}
	return nil
}
