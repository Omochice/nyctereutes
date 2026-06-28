// Package tui drives the interactive dependency-MR view that "dep" shows when
// invoked with no subcommand.
package tui

import "strings"

// Shortens every path segment except the last to its first rune, mirroring
// vim's pathshorten so a long GROUP/SUBGROUP/PROJECT path stays one column wide
// in the list (for example group/sub/project becomes g/s/project).
func pathShorten(path string) string {
	segments := strings.Split(path, "/")
	if len(segments) < 2 {
		return path
	}
	for i, segment := range segments[:len(segments)-1] {
		if segment == "" {
			continue
		}
		segments[i] = string([]rune(segment)[0])
	}
	return strings.Join(segments, "/")
}
