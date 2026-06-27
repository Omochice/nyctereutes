// Package strutil holds small string helpers shared across packages.
package strutil

import "strings"

// SplitList splits a comma-separated value, trimming each item and dropping
// empties.
func SplitList(value string) []string {
	var items []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, part)
		}
	}
	return items
}
