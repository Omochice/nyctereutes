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

func pushEvent(action string, commitCount int, refCount *int) Event {
	return Event{
		ActionName: "pushed to",
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
