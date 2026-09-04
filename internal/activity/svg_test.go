package activity

import (
	"bytes"
	"strings"
	"testing"
)

func renderSVG(t *testing.T, percents Percents) string {
	t.Helper()
	var out bytes.Buffer
	if err := WriteSVG(&out, percents); err != nil {
		t.Fatalf("WriteSVG() error = %v", err)
	}
	return out.String()
}

func TestWriteSVGLabelsEachAxisWithItsPercent(t *testing.T) {
	svg := renderSVG(t, Percents{Commits: 50, MergeRequests: 10, Issues: 10, CodeReview: 30})

	for _, want := range []string{
		"<svg", `xmlns="http://www.w3.org/2000/svg"`,
		"Commits 50%", "Merge requests 10%", "Issues 10%", "Code review 30%",
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q\n%s", want, svg)
		}
	}
}

func TestWriteSVGUsesOnlyCurrentColor(t *testing.T) {
	svg := renderSVG(t, Percents{Commits: 25, MergeRequests: 25, Issues: 25, CodeReview: 25})

	if !strings.Contains(svg, `<polygon fill="currentColor" fill-opacity="0.2" stroke="currentColor"`) {
		t.Errorf("svg does not fill the polygon with translucent currentColor\n%s", svg)
	}
	for _, attribute := range []string{"fill=", "stroke="} {
		for _, chunk := range strings.Split(svg, attribute)[1:] {
			value := strings.SplitN(chunk, `"`, 3)[1]
			if value != "currentColor" && value != "none" {
				t.Errorf("%s%q is a fixed color, want currentColor", attribute, value)
			}
		}
	}
}

func TestWriteSVGWithAllZerosIsStillAChart(t *testing.T) {
	svg := renderSVG(t, Percents{})

	if !strings.Contains(svg, `points="200,140 200,140 200,140 200,140"`) {
		t.Errorf("svg does not collapse the polygon onto the center\n%s", svg)
	}
	if !strings.Contains(svg, "Commits 0%") || !strings.Contains(svg, "</svg>") {
		t.Errorf("svg is not a complete chart\n%s", svg)
	}
}
