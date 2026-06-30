// Package tui drives the interactive dependency-MR view that "dep" shows when
// invoked with no subcommand.
package tui

import "strings"

// Shortens every path segment except the last to its first rune, mirroring
// vim's pathshorten so a long GROUP/SUBGROUP/PROJECT path stays one column wide
// in the list (for example group/sub/project becomes g/s/project).
func pathShorten(path string) string {
	segments := strings.Split(path, "/")
	// The last segment is kept in full; with a single segment the loop simply
	// does not run, so a lone project name is returned unchanged.
	for index := range len(segments) - 1 {
		if segments[index] == "" {
			continue
		}
		segments[index] = string([]rune(segments[index])[0])
	}
	return strings.Join(segments, "/")
}
