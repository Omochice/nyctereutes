package activity

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Omochice/nyctereutes/internal/glab"
)

func day(value string) time.Time {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func TestFetchWidensTheRangeAroundGitLabsExclusiveBounds(t *testing.T) {
	var calls [][]string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`[]`), nil
	})

	if _, err := Fetch(t.Context(), runner, "alice", day("2025-09-01"), day("2026-09-01")); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(calls) != 1 || len(calls[0]) != 3 || calls[0][0] != "api" || calls[0][1] != "--paginate" {
		t.Fatalf("calls = %v, want a single [api --paginate <path>]", calls)
	}
	parsed, err := url.Parse(calls[0][2])
	if err != nil {
		t.Fatalf("path %q does not parse: %v", calls[0][2], err)
	}
	if parsed.Path != "users/alice/events" {
		t.Errorf("path = %q, want users/alice/events", parsed.Path)
	}
	want := url.Values{
		"after":  {"2025-08-31"},
		"before": {"2026-09-02"},
	}
	if got := parsed.Query(); got.Encode() != want.Encode() {
		t.Errorf("query = %v, want %v", got, want)
	}
}

func TestFetchConcatenatesThePagesGlabEmits(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`[{"action_name":"opened","target_type":"MergeRequest","target_id":1}]` + "\n" +
			`[{"action_name":"approved","target_type":"MergeRequest","target_id":2}]` + "\n"), nil
	})

	events, err := Fetch(t.Context(), runner, "alice", day("2025-09-01"), day("2026-09-01"))
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(events) != 2 || events[0].TargetID != 1 || events[1].TargetID != 2 {
		t.Errorf("events = %+v, want the two events in page order", events)
	}
}

func TestCurrentUserResolvesThroughGlabAPIUser(t *testing.T) {
	var calls [][]string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`{"id":42,"username":"alice","name":"Alice"}`), nil
	})

	user, err := CurrentUser(t.Context(), runner)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}

	if user != "alice" {
		t.Errorf("CurrentUser() = %q, want alice", user)
	}
	if len(calls) != 1 || strings.Join(calls[0], " ") != "api user" {
		t.Errorf("calls = %v, want a single [api user]", calls)
	}
}

func TestCurrentUserRejectsAResponseWithoutUsername(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"id":42}`), nil
	})

	if _, err := CurrentUser(t.Context(), runner); err == nil {
		t.Error("CurrentUser() error = nil, want an error for a missing username")
	}
}
