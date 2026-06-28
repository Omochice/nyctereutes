// Package strutil holds small string helpers shared across packages.
package strutil

import "strings"

// SplitList splits a comma-separated value, trimming each item and dropping
// empties.
func SplitList(value string) []string {
	return splitTrim(value, ",")
}

// SplitLines splits a newline-separated value, trimming each item and dropping
// empties. Use this instead of SplitList for values whose items may themselves
// contain commas (for example regular expressions).
func SplitLines(value string) []string {
	return splitTrim(value, "\n")
}

func splitTrim(value, sep string) []string {
	var items []string
	for part := range strings.SplitSeq(value, sep) {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, part)
		}
	}
	return items
}
