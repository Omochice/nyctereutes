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

	for _, want := range []string{"<svg", `xmlns="http://www.w3.org/2000/svg"`,
		"Commits 50%", "Merge requests 10%", "Issues 10%", "Code review 30%",
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q\n%s", want, svg)
		}
	}
}
