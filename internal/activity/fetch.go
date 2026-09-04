package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Omochice/nyctereutes/internal/glab"
)

const perPage = 100

// GitLab's after and before parameters exclude the named day, so an inclusive
// range has to be widened by one day on each side.
const exclusiveBoundaryDays = 1

// Reads every event of user between since and until (both inclusive, date
// only) through the GitLab events API.
func Fetch(ctx context.Context, runner glab.Runner, user string, since, until time.Time) ([]Event, error) {
	query := url.Values{}
	query.Set("after", since.AddDate(0, 0, -exclusiveBoundaryDays).Format(time.DateOnly))
	query.Set("before", until.AddDate(0, 0, exclusiveBoundaryDays).Format(time.DateOnly))
	query.Set("per_page", strconv.Itoa(perPage))
	query.Set("page", "1")
	path := "users/" + url.PathEscape(user) + "/events?" + query.Encode()

	out, err := runner.Run(ctx, "api", path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}
	var events []Event
	if err := json.Unmarshal(out, &events); err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}
	return events, nil
}
