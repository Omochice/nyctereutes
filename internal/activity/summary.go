package activity

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// One axis of the JSON summary: the raw count next to its share of the
// total, so a consumer needs no arithmetic of its own.
type Axis struct {
	Count   int `json:"count"`
	Percent int `json:"percent"`
}

// The machine-readable form of one user's activity over one period. The
// dates are strings so the document reads as it was requested, without a
// time zone or clock that the command never had.
type Summary struct {
	User  string `json:"user"`
	Since string `json:"since"`
	Until string `json:"until"`
	Total int    `json:"total"`
	Axes  struct {
		Commits       Axis `json:"commits"`
		MergeRequests Axis `json:"merge_requests"`
		Issues        Axis `json:"issues"`
		CodeReview    Axis `json:"code_review"`
	} `json:"axes"`
}

// Builds the summary of counts for user between since and until (inclusive,
// date only).
func NewSummary(user string, since, until time.Time, counts Counts) Summary {
	percents := counts.Percent()
	summary := Summary{
		User:  user,
		Since: since.Format(time.DateOnly),
		Until: until.Format(time.DateOnly),
		Total: counts.Total(),
	}
	summary.Axes.Commits = Axis{Count: counts.Commits, Percent: percents.Commits}
	summary.Axes.MergeRequests = Axis{Count: counts.MergeRequests, Percent: percents.MergeRequests}
	summary.Axes.Issues = Axis{Count: counts.Issues, Percent: percents.Issues}
	summary.Axes.CodeReview = Axis{Count: counts.CodeReview, Percent: percents.CodeReview}
	return summary
}

// Writes summary as indented JSON followed by a newline, so the output ends
// cleanly whether it lands in a terminal or a file.
func WriteJSON(out io.Writer, summary Summary) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("failed to write the summary: %w", err)
	}
	return nil
}
