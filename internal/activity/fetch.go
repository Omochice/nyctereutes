package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// A GitLab account as the events API needs it addressed: some instances reject
// a username in the events path, so the numeric id is carried alongside the
// username the summary reports.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

var (
	// Reported when "glab api user" answers without a username, which would
	// otherwise leave the summary naming nobody.
	errNoUsername = errors.New("activity: current user has no username")
	// Reported when "glab api user" answers without an id, which would
	// otherwise become "users/0/events".
	errNoUserID = errors.New("activity: current user has no id")
	// Reported when the users lookup finds nobody with the given username.
	errUserNotFound = errors.New("activity: user not found")
)

// Resolves the account glab is logged in as, for reporting one's own activity
// without naming oneself.
func CurrentUser(ctx context.Context, runner glab.Runner) (User, error) {
	out, err := runner.Run(ctx, "api", "user")
	if err != nil {
		return User{}, fmt.Errorf("failed to fetch current user: %w", err)
	}
	var user User
	if err := json.Unmarshal(out, &user); err != nil {
		return User{}, fmt.Errorf("failed to parse current user: %w", err)
	}
	if user.Username == "" {
		return User{}, errNoUsername
	}
	if user.ID == 0 {
		return User{}, errNoUserID
	}
	return user, nil
}

// Resolves a username to the account it names through the users lookup, since
// the events API of some GitLab instances only accepts the numeric id.
func LookupUser(ctx context.Context, runner glab.Runner, username string) (User, error) {
	out, err := runner.Run(ctx, "api", "users?username="+url.QueryEscape(username))
	if err != nil {
		return User{}, fmt.Errorf("failed to look up user %s: %w", username, err)
	}
	var users []User
	if err := json.Unmarshal(out, &users); err != nil {
		return User{}, fmt.Errorf("failed to parse user lookup for %s: %w", username, err)
	}
	if len(users) == 0 {
		return User{}, fmt.Errorf("%w: %s", errUserNotFound, username)
	}
	return users[0], nil
}

// GitLab's after and before parameters exclude the named day, so an inclusive
// range has to be widened by one day on each side.
const exclusiveBoundaryDays = 1

// Reads every event of the user with the given id between since and until
// (both inclusive, date only) through the GitLab events API.
func Fetch(ctx context.Context, runner glab.Runner, id int, since, until time.Time) ([]Event, error) {
	query := url.Values{}
	query.Set("after", since.AddDate(0, 0, -exclusiveBoundaryDays).Format(time.DateOnly))
	query.Set("before", until.AddDate(0, 0, exclusiveBoundaryDays).Format(time.DateOnly))
	out, err := runner.Run(ctx, "api", "--paginate", "users/"+strconv.Itoa(id)+"/events?"+query.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}
	pages, err := glab.DecodePages[Event](out)
	if err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}
	return slices.Concat(pages...), nil
}
