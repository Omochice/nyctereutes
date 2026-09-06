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
var errEmptyPeriod = errors.New("--since is later than --until")

const defaultPeriodMonths = 12

// The --output file is meant to be committed and served, so it is created
// world-readable like any other source file.
const outputMode = 0o644

func parseDay(flag, value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return fallback, nil
	}
	day, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: %w", flag, err)
	}
	return day, nil
}

// Fetches the user's events for the period and writes the requested form.
func (c *Command) Execute(_ []string) error {
	ctx := context.Background()
	since, until, err := c.window()
	if err != nil {
		return err
	}

	user, err := c.user(ctx)
	if err != nil {
		return err
	}
	events, err := core.Fetch(ctx, c.runner, user.ID, since, until)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}
	counts := core.Count(events)
	if c.JSON {
		return c.write(func(out io.Writer) error {
			return core.WriteJSON(out, core.NewSummary(user.Username, since, until, counts))
		})
	}
	return c.write(func(out io.Writer) error {
		return core.WriteSVG(out, counts.Percent())
	})
}

// Resolves the account to report on: the named user when a username was
// given, otherwise the account glab is logged in as.
func (c *Command) user(ctx context.Context) (core.User, error) {
	if c.Args.Username == "" {
		user, err := core.CurrentUser(ctx, c.runner)
		if err != nil {
			return core.User{}, fmt.Errorf("resolve current user: %w", err)
		}
		return user, nil
	}
	user, err := core.LookupUser(ctx, c.runner, c.Args.Username)
	if err != nil {
		return core.User{}, fmt.Errorf("resolve user: %w", err)
	}
	return user, nil
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
			errEmptyPeriod, since.Format(time.DateOnly), until.Format(time.DateOnly))
	}
	return since, until, nil
}

// Delivers the rendered document to stdout or to the --output file.
func (c *Command) write(render func(io.Writer) error) error {
	if c.Output == "" {
		if err := render(c.inout.Stdout); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}
	// The file is rendered in memory first so a rendering failure never
	// leaves a truncated file behind.
	var document bytes.Buffer
	if err := render(&document); err != nil {
		return fmt.Errorf("render %s: %w", c.Output, err)
	}
	if err := os.WriteFile(c.Output, document.Bytes(), outputMode); err != nil {
		return fmt.Errorf("write %s: %w", c.Output, err)
	}
	return nil
}
