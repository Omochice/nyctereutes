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

	if _, err := Fetch(t.Context(), runner, 7, day("2025-09-01"), day("2026-09-01")); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(calls) != 1 || len(calls[0]) != 3 || calls[0][0] != "api" || calls[0][1] != "--paginate" {
		t.Fatalf("calls = %v, want a single [api --paginate <path>]", calls)
	}
	parsed, err := url.Parse(calls[0][2])
	if err != nil {
		t.Fatalf("path %q does not parse: %v", calls[0][2], err)
	}
	if parsed.Path != "users/7/events" {
		t.Errorf("path = %q, want users/7/events", parsed.Path)
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

	events, err := Fetch(t.Context(), runner, 7, day("2025-09-01"), day("2026-09-01"))
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

	if user.ID != 42 || user.Username != "alice" {
		t.Errorf("CurrentUser() = %+v, want id 42 and username alice", user)
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

func TestCurrentUserRejectsAResponseWithoutID(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"username":"alice"}`), nil
	})

	if _, err := CurrentUser(t.Context(), runner); err == nil {
		t.Error("CurrentUser() error = nil, want an error for a missing id")
	}
}

func TestLookupUserResolvesTheUsernameToItsID(t *testing.T) {
	var calls [][]string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`[{"id":7,"username":"alice","name":"Alice"}]`), nil
	})

	user, err := LookupUser(t.Context(), runner, "alice")
	if err != nil {
		t.Fatalf("LookupUser() error = %v", err)
	}

	if user.ID != 7 || user.Username != "alice" {
		t.Errorf("LookupUser() = %+v, want id 7 and username alice", user)
	}
	if len(calls) != 1 || strings.Join(calls[0], " ") != "api users?username=alice" {
		t.Errorf("calls = %v, want a single [api users?username=alice]", calls)
	}
}

func TestLookupUserEscapesTheUsernameInTheQuery(t *testing.T) {
	var calls [][]string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`[{"id":7,"username":"some one"}]`), nil
	})

	if _, err := LookupUser(t.Context(), runner, "some one&x=1"); err != nil {
		t.Fatalf("LookupUser() error = %v", err)
	}

	if len(calls) != 1 || calls[0][1] != "users?username=some+one%26x%3D1" {
		t.Errorf("calls = %v, want the username query-escaped", calls)
	}
}

func TestLookupUserReportsAnUnknownUsername(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})

	_, err := LookupUser(t.Context(), runner, "nobody")
	if err == nil {
		t.Fatal("LookupUser() error = nil, want an error for an empty result")
	}
	if !strings.Contains(err.Error(), "nobody") {
		t.Errorf("error = %q, want it to name the username", err)
	}
}
