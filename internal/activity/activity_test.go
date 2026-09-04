package activity

import (
	"testing"
)

func assertCounts(t *testing.T, got, want Counts) {
	t.Helper()
	if got != want {
		t.Errorf("Count() = %+v, want %+v", got, want)
	}
}

// GitLab pairs each push_data.action with its own action_name; mirroring the
// pairing keeps the fixtures shaped like real responses.
var pushActionNames = map[string]string{
	"created": "pushed new",
	"pushed":  "pushed to",
	"removed": "deleted",
}

func pushEvent(action string, commitCount int, refCount *int) Event {
	return Event{
		ActionName: pushActionNames[action],
		TargetType: "Project",
		PushData:   &PushData{Action: action, CommitCount: commitCount, RefCount: refCount},
	}
}

func TestCountAddsPushCommitCountToCommits(t *testing.T) {
	events := []Event{
		pushEvent("created", 1, nil),
		pushEvent("pushed", 3, nil),
	}

	assertCounts(t, Count(events), Counts{Commits: 4})
}

func TestCountBulkPushCountsAsOneCommit(t *testing.T) {
	refCount := 12
	events := []Event{pushEvent("pushed", 0, &refCount)}

	assertCounts(t, Count(events), Counts{Commits: 1})
}

func TestCountBranchDeletionCountsNothing(t *testing.T) {
	refCount := 2
	events := []Event{
		pushEvent("removed", 5, nil),
		pushEvent("removed", 0, &refCount),
	}

	assertCounts(t, Count(events), Counts{})
}

func TestCountOpenedMergeRequestCountsOneMergeRequest(t *testing.T) {
	events := []Event{{ActionName: "opened", TargetType: "MergeRequest", TargetID: 444342411}}

	assertCounts(t, Count(events), Counts{MergeRequests: 1})
}

func TestCountOpenedIssueCountsOneIssue(t *testing.T) {
	events := []Event{{ActionName: "opened", TargetType: "Issue", TargetID: 1770}}

	assertCounts(t, Count(events), Counts{Issues: 1})
}

func TestCountMergedAndClosedEventsCountNothing(t *testing.T) {
	events := []Event{
		{ActionName: "merged", TargetType: "MergeRequest", TargetID: 1},
		{ActionName: "closed", TargetType: "MergeRequest", TargetID: 2},
		{ActionName: "closed", TargetType: "Issue", TargetID: 3},
		{ActionName: "joined", TargetType: ""},
	}

	assertCounts(t, Count(events), Counts{})
}

func TestCountApprovedMergeRequestCountsOneCodeReview(t *testing.T) {
	events := []Event{{ActionName: "approved", TargetType: "MergeRequest", TargetID: 444342411}}

	assertCounts(t, Count(events), Counts{CodeReview: 1})
}

func commentEvent(targetType, noteableType string, noteableID int) Event {
	return Event{
		ActionName: "commented on",
		TargetType: targetType,
		Note:       &Note{NoteableType: noteableType, NoteableID: noteableID},
	}
}

func TestCountCommentOnMergeRequestCountsOneCodeReview(t *testing.T) {
	events := []Event{commentEvent("DiffNote", "MergeRequest", 444342411)}

	assertCounts(t, Count(events), Counts{CodeReview: 1})
}

func TestCountSameMergeRequestReviewedSeveralTimesCountsOnce(t *testing.T) {
	events := []Event{
		commentEvent("DiscussionNote", "MergeRequest", 444342411),
		commentEvent("DiffNote", "MergeRequest", 444342411),
		{ActionName: "approved", TargetType: "MergeRequest", TargetID: 444342411},
		commentEvent("Note", "MergeRequest", 444342412),
	}

	assertCounts(t, Count(events), Counts{CodeReview: 2})
}

func TestCountCommentOnIssueIsNotCodeReview(t *testing.T) {
	events := []Event{commentEvent("Note", "Issue", 1770)}

	assertCounts(t, Count(events), Counts{})
}

func TestPercentIsWholeNumberOverTotal(t *testing.T) {
	counts := Counts{Commits: 5, MergeRequests: 1, Issues: 0, CodeReview: 1}

	if got := counts.Total(); got != 7 {
		t.Errorf("Total() = %d, want 7", got)
	}
	want := Percents{Commits: 71, MergeRequests: 14, Issues: 0, CodeReview: 14}
	if got := counts.Percent(); got != want {
		t.Errorf("Percent() = %+v, want %+v", got, want)
	}
}

func TestPercentIsAllZeroWithoutEvents(t *testing.T) {
	if got := (Counts{}).Percent(); got != (Percents{}) {
		t.Errorf("Percent() = %+v, want all zero", got)
	}
}
