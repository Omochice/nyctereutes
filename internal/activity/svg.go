package activity

import (
	"fmt"
	"io"
)

// The chart is drawn with a radius of 100 so a percentage is also its pixel
// offset from the center. Only currentColor is used, so an inlined chart takes
// the surrounding text color while an <img> falls back to black.
const svgFormat = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 280" width="400" height="280"
  font-family="-apple-system, BlinkMacSystemFont, &quot;Segoe UI&quot;, Helvetica, Arial, sans-serif"
  font-size="12" fill="currentColor">
  <polygon points="200,40 300,140 200,240 100,140" fill="none" stroke="currentColor" stroke-opacity="0.4"/>
  <line x1="200" y1="40" x2="200" y2="240" stroke="currentColor" stroke-opacity="0.4"/>
  <line x1="100" y1="140" x2="300" y2="140" stroke="currentColor" stroke-opacity="0.4"/>
  <polygon fill="currentColor" fill-opacity="0.2" stroke="currentColor" points="200,%d %d,140 200,%d %d,140"/>
  <text x="200" y="28" text-anchor="middle">Code review %d%%</text>
  <text x="310" y="144" text-anchor="start">Issues %d%%</text>
  <text x="200" y="262" text-anchor="middle">Merge requests %d%%</text>
  <text x="90" y="144" text-anchor="end">Commits %d%%</text>
</svg>
`

// The chart's center, which the frame coordinates in svgFormat are laid out
// around.
const (
	centerX = 200
	centerY = 140
)

// Writes a diamond radar chart of the four axes as an SVG document.
func WriteSVG(out io.Writer, p Percents) error {
	_, err := fmt.Fprintf(out, svgFormat,
		centerY-p.CodeReview, centerX+p.Issues, centerY+p.MergeRequests, centerX-p.Commits,
		p.CodeReview, p.Issues, p.MergeRequests, p.Commits,
	)
	if err != nil {
		return fmt.Errorf("failed to write the chart: %w", err)
	}
	return nil
}
