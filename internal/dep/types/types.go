// Package types holds the data shared between the GitLab client (producer) and
// the command/UI layer (consumer) so the two cannot drift apart.
package types

// Reasons an MR cannot be merged.
const (
	ReasonConflict   = "conflict"
	ReasonNeedRebase = "need_rebase"
)

// A merge request with the subset of fields the dep commands consume.
type MR struct {
	IID       int    `json:"iid"`
	ProjectID int    `json:"project_id"` // numeric GitLab project ID, used for API calls
	Title     string `json:"title"`
	Author    string `json:"author"`
	Project   string `json:"project"` // GROUP/PROJECT full path
	URL       string `json:"url"`
	HeadSHA   string `json:"-"`
	CIStatus  string `json:"ci_status"` // success, pending, failure, or empty
	// UnmergeableReason is empty when the MR is mergeable; otherwise it names
	// the structural blocker (for example a conflict).
	UnmergeableReason string `json:"unmergeable_reason"`
}

// How GitLab integrates an MR into its target branch.
// The values are the names the --method flag accepts.
type MergeMethod string

// The merge methods the dep commands accept.
const (
	MergeMethodSquash MergeMethod = "squash"
	MergeMethodMerge  MergeMethod = "merge"
	MergeMethodRebase MergeMethod = "rebase"
)

// Returns every merge method, in the order the TUI cycles through them.
// A function rather than a package variable so the set cannot be mutated.
func MergeMethods() []MergeMethod {
	return []MergeMethod{MergeMethodSquash, MergeMethodMerge, MergeMethodRebase}
}

// How a merge request is merged.
// When AutoMerge is true the merge is gated on the pipeline succeeding
// (GitLab's native "merge when pipeline succeeds"); otherwise it merges immediately.
type MergeOptions struct {
	Method    MergeMethod
	AutoMerge bool
}
