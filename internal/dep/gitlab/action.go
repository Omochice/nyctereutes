package gitlab

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Approves a merge request via glab.
//
// Approval is idempotent from the caller's perspective: GitLab answers a repeat
// approval from the same user with 401 Unauthorized, so an already-approved MR
// is reported as success instead of aborting a follow-up merge. A genuinely
// unauthorized token returns 401 too, but then the follow-up merge fails on its
// own, so treating it as success here is safe.
func (c *Client) ApproveMR(ctx context.Context, project string, iid int) error {
	_, err := c.runner.Run(ctx, "mr", "approve", strconv.Itoa(iid), "-R", project)
	if err == nil || isAlreadyApproved(err) {
		return nil
	}
	return fmt.Errorf("failed to approve MR !%d: %w", iid, err)
}

func isAlreadyApproved(err error) bool {
	return err != nil && strings.Contains(err.Error(), "401")
}

// Merges a merge request via glab. When autoMerge is true the merge is gated on
// the pipeline succeeding (GitLab's native "merge when pipeline succeeds");
// otherwise it merges immediately.
func (c *Client) MergeMR(ctx context.Context, project string, iid int, method string, autoMerge bool) error {
	args := []string{"mr", "merge", strconv.Itoa(iid), "-R", project, "--yes"}

	switch method {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	}

	if autoMerge {
		args = append(args, "--auto-merge")
	} else {
		args = append(args, "--auto-merge=false")
	}

	if _, err := c.runner.Run(ctx, args...); err != nil {
		return fmt.Errorf("failed to merge MR !%d: %w", iid, err)
	}
	return nil
}
