package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// Reported when "glab api user" answers without a username, which would
// otherwise become an empty path segment in the events request.
var errNoUsername = errors.New("activity: current user has no username")

// Resolves the username of the account glab is logged in as, for reporting
// one's own activity without naming oneself.
func CurrentUser(ctx context.Context, runner glab.Runner) (string, error) {
	out, err := runner.Run(ctx, "api", "user")
	if err != nil {
		return "", fmt.Errorf("failed to fetch current user: %w", err)
	}
	var user struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(out, &user); err != nil {
		return "", fmt.Errorf("failed to parse current user: %w", err)
	}
	if user.Username == "" {
		return "", errNoUsername
	}
	return user.Username, nil
}

// GitLab's after and before parameters exclude the named day, so an inclusive
// range has to be widened by one day on each side.
const exclusiveBoundaryDays = 1

// Reads every event of user between since and until (both inclusive, date
// only) through the GitLab events API.
func Fetch(ctx context.Context, runner glab.Runner, user string, since, until time.Time) ([]Event, error) {
	query := url.Values{}
	query.Set("after", since.AddDate(0, 0, -exclusiveBoundaryDays).Format(time.DateOnly))
	query.Set("before", until.AddDate(0, 0, exclusiveBoundaryDays).Format(time.DateOnly))
	out, err := runner.Run(ctx, "api", "--paginate", "users/"+glab.EncodePath(user)+"/events?"+query.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}
	pages, err := glab.DecodePages[Event](out)
	if err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}
	return slices.Concat(pages...), nil
}
