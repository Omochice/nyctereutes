// Package gitlab finds, groups, approves and merges dependency merge requests
// by driving the glab CLI through an injected Runner.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/types"
)

// Client talks to GitLab through the glab CLI.
type Client struct {
	runner glab.Runner
}

// NewClient returns a Client backed by the given Runner.
func NewClient(runner glab.Runner) *Client {
	return &Client{runner: runner}
}

// SearchParams describes the filters used to find dependency merge requests.
type SearchParams struct {
	Group    string   // GitLab group/subgroup full path; empty means all accessible projects
	Repos    []string // explicit project paths (GROUP/PROJECT); takes precedence over Group
	Label    string
	Authors  []string
	Limit    int
	Reviewer string
}

// rawMR mirrors the subset of the GitLab merge request API we consume.
type rawMR struct {
	IID       int    `json:"iid"`
	ProjectID int    `json:"project_id"`
	Title     string `json:"title"`
	Author    struct {
		Username string `json:"username"`
	} `json:"author"`
	WebURL string `json:"web_url"`
	SHA    string `json:"sha"`
}

// SearchMRs finds open merge requests matching params. When multiple authors are
// given it runs one search per author and merges the results, deduplicating by
// project!IID. Each MR's pipeline and mergeability status is then fetched
// concurrently.
func (c *Client) SearchMRs(ctx context.Context, params SearchParams) ([]types.MR, error) {
	authors := params.Authors
	if len(authors) == 0 {
		authors = []string{""}
	}

	var allMRs []types.MR
	seen := make(map[string]bool)
	for _, author := range authors {
		mrs, err := c.searchMRsForAuthor(ctx, params, author)
		if err != nil {
			return nil, err
		}
		for _, mr := range mrs {
			key := fmt.Sprintf("%s!%d", mr.Project, mr.IID)
			if !seen[key] {
				seen[key] = true
				allMRs = append(allMRs, mr)
			}
		}
	}

	c.fillStatuses(ctx, allMRs)
	return allMRs, nil
}

// fillStatuses fetches each MR's pipeline and mergeability state concurrently
// with a bounded worker pool. Status fetch failures leave the MR's status
// fields at their zero values rather than aborting the whole search.
func (c *Client) fillStatuses(ctx context.Context, mrs []types.MR) {
	const maxWorkers = 10
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)

	for i := range mrs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			status, err := c.GetMRStatus(ctx, mrs[idx].ProjectID, mrs[idx].IID)
			if err == nil {
				mrs[idx].CIStatus = status.Pipeline
				mrs[idx].UnmergeableReason = status.UnmergeableReason
			}
		}(i)
	}
	wg.Wait()
}

// scopeEndpoints returns the API endpoints to query for the given scope.
// Explicit repos win over a group; with neither, all accessible MRs are queried.
func scopeEndpoints(params SearchParams) []string {
	if len(params.Repos) > 0 {
		endpoints := make([]string, 0, len(params.Repos))
		for _, repo := range params.Repos {
			endpoints = append(endpoints, "projects/"+url.QueryEscape(repo)+"/merge_requests")
		}
		return endpoints
	}
	if params.Group != "" {
		return []string{"groups/" + url.QueryEscape(params.Group) + "/merge_requests"}
	}
	return []string{"merge_requests?scope=all"}
}

func (c *Client) searchMRsForAuthor(ctx context.Context, params SearchParams, author string) ([]types.MR, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 200
	}

	var mrs []types.MR
	for _, endpoint := range scopeEndpoints(params) {
		endpointMRs, err := c.fetchMRs(ctx, endpoint, params, author, limit-len(mrs))
		if err != nil {
			return nil, err
		}
		mrs = append(mrs, endpointMRs...)
		if len(mrs) >= limit {
			mrs = mrs[:limit]
			break
		}
	}
	return mrs, nil
}

// fetchMRs pages through a single endpoint until limit is reached or the results
// are exhausted. Pagination is done manually (rather than glab's --paginate) so
// each response is a single JSON array we can decode.
func (c *Client) fetchMRs(ctx context.Context, endpoint string, params SearchParams, author string, limit int) ([]types.MR, error) {
	const perPage = 100

	var mrs []types.MR
	for page := 1; len(mrs) < limit; page++ {
		q := url.Values{}
		q.Set("state", "opened")
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))
		if params.Label != "" {
			q.Set("labels", params.Label)
		}
		if author != "" {
			q.Set("author_username", author)
		}
		if params.Reviewer != "" {
			q.Set("reviewer_username", params.Reviewer)
		}

		sep := "?"
		if strings.Contains(endpoint, "?") {
			sep = "&"
		}
		path := endpoint + sep + q.Encode()

		out, err := c.runner.Run(ctx, "api", path)
		if err != nil {
			return nil, fmt.Errorf("failed to search MRs: %w", err)
		}

		var raw []rawMR
		if err := json.Unmarshal(out, &raw); err != nil {
			return nil, fmt.Errorf("failed to parse search results: %w", err)
		}

		for _, r := range raw {
			mrs = append(mrs, types.MR{
				IID:       r.IID,
				ProjectID: r.ProjectID,
				Title:     r.Title,
				Author:    r.Author.Username,
				Project:   projectPathFromURL(r.WebURL),
				URL:       r.WebURL,
				HeadSHA:   r.SHA,
			})
		}

		if len(raw) < perPage {
			break
		}
	}

	if len(mrs) > limit {
		mrs = mrs[:limit]
	}
	return mrs, nil
}

// projectPathFromURL extracts GROUP/PROJECT from a merge request web URL, e.g.
// https://gitlab.com/group/sub/project/-/merge_requests/12 -> group/sub/project.
func projectPathFromURL(webURL string) string {
	u, err := url.Parse(webURL)
	if err != nil {
		return ""
	}
	path := strings.TrimPrefix(u.Path, "/")
	if idx := strings.Index(path, "/-/merge_requests"); idx >= 0 {
		return path[:idx]
	}
	return path
}

// MRStatus carries the mergeability signals read from a merge request's detail
// endpoint in a single request.
type MRStatus struct {
	// Pipeline is the head pipeline status, normalized to success, pending,
	// failure, or empty when there is no pipeline.
	Pipeline string
	// UnmergeableReason names why the MR cannot currently be merged, or is empty
	// when the MR is mergeable.
	UnmergeableReason string
}

// GetMRStatus returns the mergeability status of a merge request, reading both
// the head pipeline status and the mergeability state from one detail request.
func (c *Client) GetMRStatus(ctx context.Context, projectID, iid int) (MRStatus, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d", projectID, iid)
	out, err := c.runner.Run(ctx, "api", path)
	if err != nil {
		return MRStatus{}, err
	}
	return parseMRStatus(out)
}

func parseMRStatus(data []byte) (MRStatus, error) {
	var detail struct {
		HeadPipeline struct {
			Status string `json:"status"`
		} `json:"head_pipeline"`
		HasConflicts        bool   `json:"has_conflicts"`
		DetailedMergeStatus string `json:"detailed_merge_status"`
	}
	if err := json.Unmarshal(data, &detail); err != nil {
		return MRStatus{}, fmt.Errorf("failed to parse MR detail: %w", err)
	}
	return MRStatus{
		Pipeline:          normalizePipelineStatus(detail.HeadPipeline.Status),
		UnmergeableReason: unmergeableReason(detail.HasConflicts, detail.DetailedMergeStatus),
	}, nil
}

// unmergeableReason reports why the MR cannot be merged, or "" when it can. Only
// structural blockers count here (CI and approval state are gated elsewhere): a
// conflict, or "need_rebase", which on fast-forward projects means the branch
// trails its target.
func unmergeableReason(hasConflicts bool, detailedMergeStatus string) string {
	switch {
	case hasConflicts || detailedMergeStatus == "conflict":
		return types.ReasonConflict
	case detailedMergeStatus == "need_rebase":
		return types.ReasonNeedRebase
	}
	return ""
}

// normalizePipelineStatus maps GitLab pipeline statuses onto the three states
// the UI renders. An empty input (no pipeline) stays empty.
func normalizePipelineStatus(status string) string {
	switch status {
	case "success", "skipped":
		return "success"
	case "failed", "canceled":
		return "failure"
	case "":
		return ""
	default:
		return "pending"
	}
}
