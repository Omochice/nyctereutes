// Package activity condenses a GitLab user's event stream into the four
// contribution axes (commits, merge requests, issues, code review) that the
// activity command visualizes. It only reads events through the glab CLI.
package activity

// A push whose action is "removed" deletes a ref; GitLab still emits it as a
// push event.
const pushActionRemoved = "removed"

// The subset of a GitLab push_data payload the counting rules look at. The
// JSON tags are snake_case because they mirror GitLab's API.
type PushData struct {
	CommitCount int    `json:"commit_count"`
	Action      string `json:"action"`
	// Null for an ordinary push and set for a bulk push, so a pointer keeps
	// the two cases apart.
	RefCount *int `json:"ref_count"`
}

// The subset of a GitLab user event the counting rules look at. Payload
// objects that only some event kinds carry are pointers so their absence is
// distinguishable from zero values.
type Event struct {
	ActionName string    `json:"action_name"`
	TargetType string    `json:"target_type"`
	TargetID   int       `json:"target_id"`
	PushData   *PushData `json:"push_data"`
}

// The four contribution axes of one user over one period.
type Counts struct {
	Commits       int
	MergeRequests int
	Issues        int
	CodeReview    int
}

// Folds events into the four axes.
func Count(events []Event) Counts {
	var counts Counts
	for _, event := range events {
		if event.PushData != nil {
			counts.Commits += commitsOf(event.PushData)
		}
	}
	return counts
}

// A bulk push carries no commit total, only the number of refs, so it is
// counted as a single contribution rather than as zero commits. Removal is
// checked first because a bulk deletion also carries a ref count.
func commitsOf(push *PushData) int {
	if push.Action == pushActionRemoved {
		return 0
	}
	if push.RefCount != nil {
		return 1
	}
	return push.CommitCount
}
