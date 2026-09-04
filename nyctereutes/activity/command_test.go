package activity_test

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Omochice/nyctereutes/cli"
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
	if len(fake.paths) < 2 || fake.paths[0] != "user" || !strings.HasPrefix(fake.paths[1], "users/me/events?") {
		t.Errorf("paths = %v, want the current user resolved and then their events fetched", fake.paths)
	}
}

func TestActivityWithAUsernameReportsThatUser(t *testing.T) {
	fake := &fakeGlab{events: somePushes}

	exit, stdout, stderr := runWithRunner(fake, "activity", "someone/else", "--since", "2025-09-01", "--until", "2026-09-01")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if !strings.HasPrefix(stdout, "<svg") {
		t.Errorf("stdout is not the chart\n%s", stdout)
	}
	if len(fake.paths) == 0 || !strings.HasPrefix(fake.paths[0], "users/someone%2Felse/events?") {
		t.Errorf("paths = %v, want the named user's events fetched first, without resolving the current user", fake.paths)
	}
}

func TestActivityDefaultsToTheTwelveMonthsEndingToday(t *testing.T) {
	fake := &fakeGlab{events: somePushes}
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := activity.New(&cli.ProcInout{Stdin: strings.NewReader(""), Stdout: outBuf, Stderr: errBuf}, fake)
	activity.SetNow(cmd, func() time.Time {
		return time.Date(2026, time.September, 4, 23, 30, 0, 0, time.FixedZone("JST", 9*60*60))
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
		t.Errorf("after = %s, want 2025-09-03 (the day before 12 months ago, in UTC)", got)
	}
	if got := query.Get("before"); got != "2026-09-05" {
		t.Errorf("before = %s, want 2026-09-05 (the day after today, in UTC)", got)
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
