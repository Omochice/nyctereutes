package activity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSONCarriesUserPeriodTotalAndEveryAxis(t *testing.T) {
	counts := Counts{Commits: 5, MergeRequests: 1, Issues: 0, CodeReview: 1}
	summary := NewSummary("alice", day("2025-09-01"), day("2026-09-01"), counts)

	var out bytes.Buffer
	if err := WriteJSON(&out, summary); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	if !strings.HasSuffix(out.String(), "\n") {
		t.Error("output does not end with a newline")
	}
	for _, key := range []string{
		`"user"`, `"since"`, `"until"`, `"total"`, `"axes"`,
		`"commits"`, `"merge_requests"`, `"issues"`, `"code_review"`, `"count"`, `"percent"`,
	} {
		if !strings.Contains(out.String(), key) {
			t.Errorf("output missing key %s\n%s", key, out.String())
		}
	}
	var decoded Summary
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	want := Summary{
		User: "alice", Since: "2025-09-01", Until: "2026-09-01", Total: 7,
		Axes: Axes{
			Commits:       Axis{Count: 5, Percent: 71},
			MergeRequests: Axis{Count: 1, Percent: 14},
			Issues:        Axis{Count: 0, Percent: 0},
			CodeReview:    Axis{Count: 1, Percent: 14},
		},
	}
	if decoded != want {
		t.Errorf("decoded = %+v, want %+v", decoded, want)
	}
}
