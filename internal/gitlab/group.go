package gitlab

import (
	"github.com/Omochice/nyctereutes/internal/parser"
	"github.com/Omochice/nyctereutes/internal/types"
)

// GroupMRs buckets merge requests by the "package@version" key parsed from each
// title. customPatterns override the built-in title patterns.
func GroupMRs(mrs []types.MR, customPatterns []string) map[string][]types.MR {
	groups := make(map[string][]types.MR)
	for _, mr := range mrs {
		key := parser.ParseTitle(mr.Title, customPatterns).GroupKey()
		groups[key] = append(groups[key], mr)
	}
	return groups
}
