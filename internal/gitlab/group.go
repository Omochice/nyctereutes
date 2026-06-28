package gitlab

import (
	"fmt"
	"regexp"

	"github.com/Omochice/nyctereutes/internal/parser"
	"github.com/Omochice/nyctereutes/internal/types"
)

// Buckets merge requests by the "package@version" key parsed from each title.
// customPatterns override the built-in title patterns and are compiled once
// here rather than per merge request; invalid patterns are skipped.
//
// An MR whose title cannot be parsed gets a unique fallback key so that a bulk
// approve/merge against a group never sweeps up unrelated, unparsed MRs.
func GroupMRs(mrs []types.MR, customPatterns []string) map[string][]types.MR {
	compiled := make([]*regexp.Regexp, 0, len(customPatterns))
	for _, p := range customPatterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	groups := make(map[string][]types.MR)
	for _, mergeRequest := range mrs {
		var key string
		if update, ok := parser.ParseTitle(mergeRequest.Title, compiled); ok {
			key = update.GroupKey()
		} else {
			key = fmt.Sprintf("unparsed:%s!%d", mergeRequest.Project, mergeRequest.IID)
		}
		groups[key] = append(groups[key], mergeRequest)
	}
	return groups
}
