package parser

import (
	"regexp"
	"testing"
)

func TestParseTitle(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		want   string // GroupKey form: package@version, when parsed
		wantOK bool
	}{
		{
			name:   "bump from-to",
			title:  "Bump lodash from 4.17.15 to 4.17.21",
			want:   "lodash@4.17.21",
			wantOK: true,
		},
		{
			name:   "update dependency to",
			title:  "Update dependency typescript to 5.6.0",
			want:   "typescript@5.6.0",
			wantOK: true,
		},
		{
			name:   "catch-all X to semver",
			title:  "chore(deps): eslint to v8.57.0",
			want:   "eslint@8.57.0",
			wantOK: true,
		},
		{
			name:   "single-segment version",
			title:  "Bump go from 1 to v2",
			want:   "go@2",
			wantOK: true,
		},
		{
			name:   "prerelease version",
			title:  "Update dependency eslint to 9.0.0-beta.1",
			want:   "eslint@9.0.0-beta.1",
			wantOK: true,
		},
		{
			name:   "unparseable title",
			title:  "Refactor the build pipeline",
			wantOK: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := ParseTitle(testCase.title, nil)
			if ok != testCase.wantOK {
				t.Fatalf("ParseTitle(%q) ok = %v, want %v", testCase.title, ok, testCase.wantOK)
			}
			if ok && got.GroupKey() != testCase.want {
				t.Errorf("ParseTitle(%q).GroupKey() = %q, want %q", testCase.title, got.GroupKey(), testCase.want)
			}
		})
	}
}

func TestParseTitleCustomPatternsTakePrecedence(t *testing.T) {
	// A custom pattern matches a title that the default patterns would parse
	// differently, proving custom patterns are tried first.
	custom := []*regexp.Regexp{regexp.MustCompile(`(?i)renovate:\s+(\S+)\s+->\s+(\S+)`)}
	got, ok := ParseTitle("renovate: my-pkg -> 2.0.0", custom)
	if !ok {
		t.Fatal("ParseTitle with custom pattern did not match")
	}
	if want := "my-pkg@2.0.0"; got.GroupKey() != want {
		t.Errorf("ParseTitle with custom pattern = %q, want %q", got.GroupKey(), want)
	}
}

func TestParseTitleRejectsEmptyCapture(t *testing.T) {
	// The custom pattern matches "[] 1.2.3" but leaves the package capture
	// empty; that must not count as a parsed update (no built-in pattern
	// matches this title either).
	custom := []*regexp.Regexp{regexp.MustCompile(`^\[(\w*)\]\s+(v?\d\S*)`)}
	if got, ok := ParseTitle("[] 1.2.3", custom); ok {
		t.Errorf("empty package capture parsed as %q, want not ok", got.GroupKey())
	}
}
