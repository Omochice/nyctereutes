// Package types holds the data shared between the GitLab client (producer) and
// the command/UI layer (consumer) so the two cannot drift apart.
package types

// Reasons an MR cannot be merged.
const (
	ReasonConflict   = "conflict"
	ReasonNeedRebase = "need_rebase"
)

// MR is a merge request with the subset of fields the dep commands consume.
type MR struct {
	IID       int    `json:"iid"`
	ProjectID int    `json:"project_id"` // numeric GitLab project ID, used for API calls
	Title     string `json:"title"`
	Author    string `json:"author"`
	Project   string `json:"project"` // GROUP/PROJECT full path
	URL       string `json:"url"`
	HeadSHA   string `json:"-"`         // head commit SHA
	CIStatus  string `json:"ci_status"` // success, pending, failure, or empty
	// UnmergeableReason is empty when the MR is mergeable; otherwise it names
	// the structural blocker (for example a conflict).
	UnmergeableReason string `json:"unmergeable_reason"`
}
