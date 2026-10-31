package activity

import "time"

// Pins "today" so the default period is deterministic.
func SetNow(c *Command, now func() time.Time) {
	c.now = now
}
