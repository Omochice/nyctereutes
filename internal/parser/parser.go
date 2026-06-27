// Package parser extracts the dependency name and target version from a merge
// request title so that updates of the same package@version can be grouped.
package parser

import "regexp"

type PackageUpdate struct {
	Package   string
	ToVersion string
}

// patterns are tried in order; the first match wins. They move from the most
// specific dependency-bot phrasing to a permissive catch-all.
var patterns = []*regexp.Regexp{
	// "Bump/Update PACKAGE from X to VERSION" (Dependabot/Renovate "from...to").
	regexp.MustCompile(`(?i)(?:bump|update)[:\s]+([^\s]+)\s+from\s+[^\s]+\s+to\s+v?(\d+\.\d+(?:\.\d+)?)`),
	// "Update [dependency] PACKAGE to VERSION".
	regexp.MustCompile(`(?i)update\s+(?:dependency\s+)?([^\s]+)\s+to\s+v?(\d+\.\d+(?:\.\d+)?)`),
	// Catch-all: the last word before "to" and a following semver.
	regexp.MustCompile(`(?i)([^\s:]+)\s+to\s+v?(\d+\.\d+(?:\.\d+)?)`),
}

// ParseTitle extracts the package and target version from a merge request title.
// Custom patterns are tried before the built-in ones, so a project can override
// parsing for its own title conventions; callers pass them pre-compiled so the
// regexps are built once rather than per title. When nothing matches, both
// fields are "unknown".
func ParseTitle(title string, customPatterns []*regexp.Regexp) PackageUpdate {
	for _, re := range customPatterns {
		if u, ok := match(re, title); ok {
			return u
		}
	}

	for _, re := range patterns {
		if u, ok := match(re, title); ok {
			return u
		}
	}

	return PackageUpdate{Package: "unknown", ToVersion: "unknown"}
}

func match(re *regexp.Regexp, title string) (PackageUpdate, bool) {
	m := re.FindStringSubmatch(title)
	if len(m) != 3 {
		return PackageUpdate{}, false
	}
	return PackageUpdate{Package: m[1], ToVersion: m[2]}, true
}

// GroupKey is the "package@version" key used to bucket updates of the same
// dependency to the same version.
func (u PackageUpdate) GroupKey() string {
	return u.Package + "@" + u.ToVersion
}
