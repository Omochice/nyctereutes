package activity

import (
	"fmt"
	"io"
	"text/template"
)

// The axis names as they appear in the chart. The JSON summary keys its axes
// in snake_case instead, so a machine consumer never depends on this wording.
const (
	LabelCommits       = "Commits"
	LabelMergeRequests = "Merge requests"
	LabelIssues        = "Issues"
	LabelCodeReview    = "Code review"
)

// The chart is drawn with a radius of 100 so every vertex offset equals its
// percentage and the template needs nothing but integers. Only currentColor
// is used, so an inlined chart takes the surrounding text color; embedded as
// an image it falls back to the browser's default, black.
const svgTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 280" width="400" height="280"
  font-family="-apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Helvetica, Arial, sans-serif"
  font-size="12" fill="currentColor">
  <polygon points="200,40 300,140 200,240 100,140" fill="none" stroke="currentColor" stroke-opacity="0.4"/>
  <line x1="200" y1="40" x2="200" y2="240" stroke="currentColor" stroke-opacity="0.4"/>
  <line x1="100" y1="140" x2="300" y2="140" stroke="currentColor" stroke-opacity="0.4"/>
  <polygon fill="currentColor" fill-opacity="0.2" stroke="currentColor" points="{{.Points}}"/>
  <text x="200" y="28" text-anchor="middle">{{.CodeReviewLabel}} {{.CodeReview}}%</text>
  <text x="310" y="144" text-anchor="start">{{.IssuesLabel}} {{.Issues}}%</text>
  <text x="200" y="262" text-anchor="middle">{{.MergeRequestsLabel}} {{.MergeRequests}}%</text>
  <text x="90" y="144" text-anchor="end">{{.CommitsLabel}} {{.Commits}}%</text>
</svg>
`

// The chart's center, from which each vertex is offset by its percentage:
// code review upward, issues rightward, merge requests downward, commits
// leftward.
const (
	centerX = 200
	centerY = 140
)

// Lays out the data polygon's vertices clockwise from the top.
func polygonPoints(percents Percents) string {
	return fmt.Sprintf("%d,%d %d,%d %d,%d %d,%d",
		centerX, centerY-percents.CodeReview,
		centerX+percents.Issues, centerY,
		centerX, centerY+percents.MergeRequests,
		centerX-percents.Commits, centerY,
	)
}

// Writes a diamond radar chart of the four axes as an SVG document.
func WriteSVG(out io.Writer, percents Percents) error {
	chart, err := template.New("svg").Parse(svgTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse the chart template: %w", err)
	}
	data := struct {
		Percents

		Points                                                         string
		CommitsLabel, MergeRequestsLabel, IssuesLabel, CodeReviewLabel string
	}{
		Percents:           percents,
		Points:             polygonPoints(percents),
		CommitsLabel:       LabelCommits,
		MergeRequestsLabel: LabelMergeRequests,
		IssuesLabel:        LabelIssues,
		CodeReviewLabel:    LabelCodeReview,
	}
	if err := chart.Execute(out, data); err != nil {
		return fmt.Errorf("failed to write the chart: %w", err)
	}
	return nil
}
