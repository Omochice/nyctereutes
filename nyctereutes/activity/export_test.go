package activity

import "time"

// Substitutes the clock the default period is derived from, so a test can pin
// "today" instead of depending on the day it runs. The field stays unexported
// so production callers cannot swap it, and this seam lives in a test file, so
// it exists only in the test binary.
func SetNow(c *Command, now func() time.Time) {
	c.now = now
}
