// The "activity" subcommand, which charts a user's GitLab contributions.
package activity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Omochice/nyctereutes/cli"
	core "github.com/Omochice/nyctereutes/internal/activity"
	"github.com/Omochice/nyctereutes/internal/glab"
)

// The command go-flags parses "activity" into.
type Command struct {
	Since  string `long:"since" value-name:"YYYY-MM-DD" description:"First day of the period (default: 12 months ago)"`
	Until  string `long:"until" value-name:"YYYY-MM-DD" description:"Last day of the period (default: today)"`
	JSON   bool   `long:"json" description:"Write the JSON summary instead of the SVG chart"`
	Output string `short:"o" long:"output" value-name:"PATH" description:"Write to this file instead of stdout"`
	Args   struct {
		Username string `positional-arg-name:"username" description:"GitLab username (default: the logged-in user)"`
	} `positional-args:"yes"`

	inout  *cli.ProcInout
	runner glab.Runner
	// Supplies "today" for the default period; it is a field so tests can pin
	// the day instead of depending on when they run.
	now func() time.Time
}

// Builds the command wired to the given streams and glab runner, so a caller
// can inject a fake runner instead of the real CLI.
func New(inout *cli.ProcInout, runner glab.Runner) *Command {
	return &Command{inout: inout, runner: runner, now: time.Now}
}

// Reported when --since names a later day than --until, a period that could
// only chart nothing.
var ErrEmptyPeriod = errors.New("--since is later than --until")

// How far back the period reaches when --since is not given.
const defaultPeriodMonths = 12

// The --output file is meant to be committed and served, so it is created
// world-readable like any other source file.
const outputMode = 0o644

// Parses a period flag, or falls back to the given day when the flag was
// not set.
func parseDay(flag, value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return fallback, nil
	}
	day, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return day, fmt.Errorf("invalid %s: %w", flag, err)
	}
	return day, nil
}

// Fetches the user's events for the period and writes the chart.
func (c *Command) Execute(_ []string) error {
	ctx := context.Background()
	since, until, err := c.window()
	if err != nil {
		return err
	}

	user := c.Args.Username
	if user == "" {
		user, err = core.CurrentUser(ctx, c.runner)
		if err != nil {
			return fmt.Errorf("resolve current user: %w", err)
		}
	}
	events, err := core.Fetch(ctx, c.runner, user, since, until)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}
	return c.write(core.NewSummary(user, since, until, core.Count(events)))
}

// Resolves the period flags into inclusive date-only bounds. Today is taken
// in UTC so the same invocation yields the same period in every time zone.
func (c *Command) window() (since, until time.Time, err error) {
	now := c.now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	since, err = parseDay("--since", c.Since, today.AddDate(0, -defaultPeriodMonths, 0))
	if err != nil {
		return since, until, err
	}
	until, err = parseDay("--until", c.Until, today)
	if err != nil {
		return since, until, err
	}
	if since.After(until) {
		return since, until, fmt.Errorf("%w: %s > %s",
			ErrEmptyPeriod, since.Format(time.DateOnly), until.Format(time.DateOnly))
	}
	return since, until, nil
}

// Delivers the rendered summary to stdout or to the --output file. The
// document is rendered in memory first so a rendering failure never leaves a
// truncated file behind.
func (c *Command) write(summary core.Summary) error {
	var document bytes.Buffer
	if err := c.render(&document, summary); err != nil {
		return err
	}
	if c.Output == "" {
		if _, err := c.inout.Stdout.Write(document.Bytes()); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(c.Output, document.Bytes(), outputMode); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// Writes the summary in the form the flags selected.
func (c *Command) render(out io.Writer, summary core.Summary) error {
	if c.JSON {
		if err := core.WriteJSON(out, summary); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
		return nil
	}
	if err := core.WriteSVG(out, summary.Percents()); err != nil {
		return fmt.Errorf("write chart: %w", err)
	}
	return nil
}
