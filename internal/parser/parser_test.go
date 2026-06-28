package parser

import (
	"regexp"
	"testing"
)

func TestParseTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string // GroupKey form: package@version
	}{
		{
			name:  "bump from-to",
			title: "Bump lodash from 4.17.15 to 4.17.21",
			want:  "lodash@4.17.21",
		},
		{
			name:  "update dependency to",
			title: "Update dependency typescript to 5.6.0",
			want:  "typescript@5.6.0",
		},
		{
			name:  "catch-all X to semver",
			title: "chore(deps): eslint to v8.57.0",
			want:  "eslint@8.57.0",
		},
		{
			name:  "single-segment version",
			title: "Bump go from 1 to v2",
			want:  "go@2",
		},
		{
			name:  "prerelease version",
			title: "Update dependency eslint to 9.0.0-beta.1",
			want:  "eslint@9.0.0-beta.1",
		},
		{
			name:  "unparseable title",
			title: "Refactor the build pipeline",
			want:  "unknown@unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTitle(tt.title, nil).GroupKey()
			if got != tt.want {
				t.Errorf("ParseTitle(%q).GroupKey() = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestParseTitleCustomPatternsTakePrecedence(t *testing.T) {
	// A custom pattern matches a title that the default patterns would parse
	// differently, proving custom patterns are tried first.
	custom := []*regexp.Regexp{regexp.MustCompile(`(?i)renovate:\s+(\S+)\s+->\s+(\S+)`)}
	got := ParseTitle("renovate: my-pkg -> 2.0.0", custom).GroupKey()
	if want := "my-pkg@2.0.0"; got != want {
		t.Errorf("ParseTitle with custom pattern = %q, want %q", got, want)
	}
}
