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
// only) through the GitLab events API. Pages are requested one at a time
// (rather than with glab's --paginate) so each response is a single JSON
// array, and paging stops at the first empty page because the API reports no
// total.
func Fetch(ctx context.Context, runner glab.Runner, user string, since, until time.Time) ([]Event, error) {
	query := url.Values{}
	query.Set("after", since.AddDate(0, 0, -exclusiveBoundaryDays).Format(time.DateOnly))
	query.Set("before", until.AddDate(0, 0, exclusiveBoundaryDays).Format(time.DateOnly))
	query.Set("per_page", strconv.Itoa(perPage))
	endpoint := "users/" + url.PathEscape(user) + "/events?"

	var events []Event
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		out, err := runner.Run(ctx, "api", endpoint+query.Encode())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch events: %w", err)
		}
		var batch []Event
		if err := json.Unmarshal(out, &batch); err != nil {
			return nil, fmt.Errorf("failed to parse events: %w", err)
		}
		if len(batch) == 0 {
			return events, nil
		}
		events = append(events, batch...)
	}
}
