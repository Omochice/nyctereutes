package gitlab

import (
	"regexp"

	"github.com/Omochice/nyctereutes/internal/parser"
	"github.com/Omochice/nyctereutes/internal/types"
)

// GroupMRs buckets merge requests by the "package@version" key parsed from each
// title. customPatterns override the built-in title patterns and are compiled
// once here rather than per merge request; invalid patterns are skipped.
func GroupMRs(mrs []types.MR, customPatterns []string) map[string][]types.MR {
	compiled := make([]*regexp.Regexp, 0, len(customPatterns))
	for _, p := range customPatterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	groups := make(map[string][]types.MR)
	for _, mr := range mrs {
		key := parser.ParseTitle(mr.Title, compiled).GroupKey()
		groups[key] = append(groups[key], mr)
	}
	return groups
}
