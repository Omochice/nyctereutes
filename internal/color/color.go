// The color scale every nyctereutes surface draws from, together with the
// decision of whether a given writer may receive color at all.
package color

import (
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

// ANSI 256-color indices forming the shared scale. They are named after the hue
// rather than after a meaning because each surface assigns its own: the infra
// diff reads green as an addition, the dep TUI reads it as a passing pipeline.
const (
	Green  = "42"
	Red    = "196"
	Yellow = "226"
	Gray   = "240"
)

// Reports whether ANSI color may be written to w: only when w is a real
// terminal and NO_COLOR is absent, so piped or captured output stays plain.
func Enabled(w io.Writer) bool {
	if disabled() {
		return false
	}
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}

// Reports whether the NO_COLOR convention forbids color: the variable is
// present, regardless of its value, even an empty one.
func disabled() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}
