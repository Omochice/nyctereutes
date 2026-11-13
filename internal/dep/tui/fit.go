package tui

import "strings"

// Cuts content to its top height lines, counting lines the way bubbletea
// measures a frame, so an inline frame never outgrows the terminal.
// A height of zero means the terminal size is not known yet and leaves
// content as is.
//
// Returns content unchanged when it already fits.
func fitHeight(content string, height int) string {
	if height <= 0 {
		return content
	}
	end := 0
	for range height {
		next := strings.IndexByte(content[end:], '\n')
		if next < 0 {
			return content
		}
		end += next + 1
	}
	return content[:end-1]
}
