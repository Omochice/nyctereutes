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

	if len(calls) != 1 || len(calls[0]) != 2 || calls[0][0] != "api" {
		t.Fatalf("calls = %v, want a single [api <path>]", calls)
	}
	parsed, err := url.Parse(calls[0][1])
	if err != nil {
		t.Fatalf("path %q does not parse: %v", calls[0][1], err)
	}
	if parsed.Path != "users/alice/events" {
		t.Errorf("path = %q, want users/alice/events", parsed.Path)
	}
	want := url.Values{
		"after":    {"2025-08-31"},
		"before":   {"2026-09-02"},
		"per_page": {"100"},
		"page":     {"1"},
	}
	if got := parsed.Query(); got.Encode() != want.Encode() {
		t.Errorf("query = %v, want %v", got, want)
	}
}

func TestFetchReadsPagesUntilAnEmptyOne(t *testing.T) {
	pages := map[string]string{
		"1": `[{"action_name":"opened","target_type":"MergeRequest","target_id":1}]`,
		"2": `[{"action_name":"approved","target_type":"MergeRequest","target_id":2}]`,
		"3": `[]`,
	}
	var requested []string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		parsed, err := url.Parse(args[len(args)-1])
		if err != nil {
			return nil, err
		}
		page := parsed.Query().Get("page")
		requested = append(requested, page)
		return []byte(pages[page]), nil
	})

	events, err := Fetch(t.Context(), runner, "alice", day("2025-09-01"), day("2026-09-01"))
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(events) != 2 || events[0].TargetID != 1 || events[1].TargetID != 2 {
		t.Errorf("events = %+v, want the two events in page order", events)
	}
	if got := strings.Join(requested, ","); got != "1,2,3" {
		t.Errorf("requested pages = %s, want 1,2,3", got)
	}
}
