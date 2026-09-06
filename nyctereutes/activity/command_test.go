package activity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Omochice/nyctereutes/cli"
	core "github.com/Omochice/nyctereutes/internal/activity"
	"github.com/Omochice/nyctereutes/nyctereutes/activity"
)

func TestActivityWithNoArgumentsWritesTheCurrentUsersChart(t *testing.T) {
	fake := &fakeGlab{events: somePushes}

	exit, stdout, stderr := runWithRunner(fake, "activity", "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if !strings.HasPrefix(stdout, "<svg") || !strings.Contains(stdout, "Commits 75%") {
		t.Errorf("stdout is not the chart of 3 commits and 1 merge request\n%s", stdout)
	}
	if len(fake.paths) < 2 || fake.paths[0] != "user" || !strings.HasPrefix(fake.paths[1], "users/42/events?") {
		t.Errorf("paths = %v, want the current user resolved and then their events fetched", fake.paths)
	}
}

func TestActivityWithAUsernameReportsThatUser(t *testing.T) {
	fake := &fakeGlab{events: somePushes}

	exit, stdout, stderr := runWithRunner(fake,
		"activity", "alice", "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if !strings.HasPrefix(stdout, "<svg") {
		t.Errorf("stdout is not the chart\n%s", stdout)
	}
	lookedUp := len(fake.paths) >= 2 && fake.paths[0] == "users?username=alice"
	if !lookedUp || !strings.HasPrefix(fake.paths[1], "users/7/events?") {
		t.Errorf("paths = %v, want the username looked up and then that id's events fetched,"+
			" without resolving the current user", fake.paths)
	}
}

func TestActivityRejectsAnUnknownUsername(t *testing.T) {
	fake := &fakeGlab{events: somePushes}

	exit, stdout, stderr := runWithRunner(fake,
		"activity", "nobody", "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", exit, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no chart for an unknown user", stdout)
	}
	if !strings.Contains(stderr, "nobody") {
		t.Errorf("stderr = %q, want it to name the unknown username", stderr)
	}
	if len(fake.paths) != 1 {
		t.Errorf("paths = %v, want no events request for an unknown user", fake.paths)
	}
}

func TestActivityDefaultsToTheTwelveMonthsEndingToday(t *testing.T) {
	fake := &fakeGlab{events: somePushes}
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := activity.New(&cli.ProcInout{Stdin: strings.NewReader(""), Stdout: outBuf, Stderr: errBuf}, fake)
	activity.SetNow(cmd, func() time.Time {
		return time.Date(2026, time.September, 5, 5, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	})

	if err := cmd.Execute(nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(fake.paths) < 2 {
		t.Fatalf("paths = %v, want an events request", fake.paths)
	}
	parsed, err := url.Parse(fake.paths[1])
	if err != nil {
		t.Fatalf("path %q does not parse: %v", fake.paths[1], err)
	}
	query := parsed.Query()
	if got := query.Get("after"); got != "2025-09-03" {
		t.Errorf("after = %s, want 2025-09-03 (the day before 12 months ago, dated in UTC where it is still the 4th)", got)
	}
	if got := query.Get("before"); got != "2026-09-05" {
		t.Errorf("before = %s, want 2026-09-05 (the day after today, dated in UTC where it is still the 4th)", got)
	}
}

func TestActivityJSONWritesTheSummaryInsteadOfTheChart(t *testing.T) {
	fake := &fakeGlab{events: somePushes}

	exit, stdout, stderr := runWithRunner(fake, "activity", "--json", "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	var summary core.Summary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if summary.User != "me" || summary.Since != "2025-09-01" || summary.Until != "2026-09-01" || summary.Total != 4 {
		t.Errorf("summary = %+v, want user me over 2025-09-01..2026-09-01 with 4 contributions", summary)
	}
	if summary.Axes.Commits.Count != 3 || summary.Axes.MergeRequests.Percent != 25 {
		t.Errorf("axes = %+v, want 3 commits and a 25%% merge request share", summary.Axes)
	}
}

func TestActivityOutputWritesToTheFileInsteadOfStdout(t *testing.T) {
	fake := &fakeGlab{events: somePushes}
	path := filepath.Join(t.TempDir(), "activity.svg")

	exit, stdout, stderr := runWithRunner(fake,
		"activity", "--output", path, "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing when a file is named", stdout)
	}
	written, err := os.ReadFile(path) //nolint:gosec // G304: the test's own temporary file
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.HasPrefix(string(written), "<svg") {
		t.Errorf("file is not the chart\n%s", written)
	}
}

func TestActivityOutputWithJSONWritesTheSummaryToTheFile(t *testing.T) {
	fake := &fakeGlab{events: somePushes}
	path := filepath.Join(t.TempDir(), "activity.json")

	exit, _, stderr := runWithRunner(fake,
		"activity", "--json", "--output", path, "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	written, err := os.ReadFile(path) //nolint:gosec // G304: the test's own temporary file
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !json.Valid(written) {
		t.Errorf("file is not JSON\n%s", written)
	}
}

func TestActivityReportsAGlabFailureOnStderr(t *testing.T) {
	fake := &fakeGlab{err: errors.New("glab api user: exit status 1\nHTTP 401 Unauthorized")}

	exit, stdout, stderr := runWithRunner(fake, "activity")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no chart after a failure", stdout)
	}
	if !strings.Contains(stderr, "HTTP 401 Unauthorized") {
		t.Errorf("stderr = %q, want glab's own diagnostic kept", stderr)
	}
}

func TestActivityWithNoEventsStillWritesAnEmptyChart(t *testing.T) {
	fake := &fakeGlab{events: `[]`}

	exit, stdout, stderr := runWithRunner(fake, "activity", "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	for _, want := range []string{"<svg", "Commits 0%", "Code review 0%"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

func TestActivityRejectsAnUnusablePeriod(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		wants string
	}{
		{"malformed since", []string{"--since", "2025/09/01", "--until", "2026-09-01"}, "--since"},
		{"malformed until", []string{"--since", "2025-09-01", "--until", "tomorrow"}, "--until"},
		{"since after until", []string{"--since", "2026-09-02", "--until", "2026-09-01"}, "2026-09-02"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeGlab{events: somePushes}

			exit, stdout, stderr := runWithRunner(fake, append([]string{"activity"}, testCase.args...)...)

			if exit != 1 {
				t.Fatalf("exit = %d, want 1", exit)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want no chart for a rejected period", stdout)
			}
			if !strings.Contains(stderr, testCase.wants) {
				t.Errorf("stderr = %q, want it to mention %q", stderr, testCase.wants)
			}
			if len(fake.paths) != 0 {
				t.Errorf("paths = %v, want no glab call for a rejected period", fake.paths)
			}
		})
	}
}
