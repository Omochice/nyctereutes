package dep

import "github.com/Omochice/nyctereutes/internal/dep/tui"

// Substitutes the TUI launcher so a test can observe the launch instead of
// driving a real terminal program. The field stays unexported so production
// callers cannot swap it, and this seam lives in a test file, so it exists only
// in the test binary and never widens the package's API.
func SetLaunch(c *Command, launch func(tui.Model) error) {
	c.launch = launch
}
