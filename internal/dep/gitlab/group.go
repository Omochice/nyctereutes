package gitlab

import (
	"fmt"
	"regexp"

	"github.com/Omochice/nyctereutes/internal/dep/parser"
	"github.com/Omochice/nyctereutes/internal/dep/types"
)

// Buckets merge requests by the "package@version" key parsed from each title.
// customPatterns override the built-in title patterns and are compiled once
// here rather than per merge request; invalid patterns are skipped.
//
// An MR whose title cannot be parsed gets a unique fallback key so that a bulk
// approve/merge against a group never sweeps up unrelated, unparsed MRs.
func GroupMRs(mrs []types.MR, customPatterns []string) map[string][]types.MR {
	compiled := compilePatterns(customPatterns)

	groups := make(map[string][]types.MR)
	for _, mergeRequest := range mrs {
		key := groupKeyOf(mergeRequest, compiled)
		groups[key] = append(groups[key], mergeRequest)
	}
	return groups
}

// Returns a function mapping an MR to its package@version key, using the same
// bucketing (and unparsed fallback) as [GroupMRs]. The patterns are compiled
// once up front so repeated calls (such as the TUI's group filter over every
// visible MR) do not recompile them per MR.
func GroupKeyFunc(customPatterns []string) func(types.MR) string {
	compiled := compilePatterns(customPatterns)
	return func(mr types.MR) string {
		return groupKeyOf(mr, compiled)
	}
}

// Compiles the custom title patterns, skipping invalid ones.
func compilePatterns(customPatterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(customPatterns))
	for _, p := range customPatterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// Derives one MR's group key from already-compiled patterns. An MR whose title
// cannot be parsed gets a unique fallback key so a bulk action against a group
// never sweeps up unrelated, unparsed MRs.
func groupKeyOf(mr types.MR, compiled []*regexp.Regexp) string {
	if update, ok := parser.ParseTitle(mr.Title, compiled); ok {
		return update.GroupKey()
	}
	return fmt.Sprintf("unparsed:%s!%d", mr.Project, mr.IID)
}
