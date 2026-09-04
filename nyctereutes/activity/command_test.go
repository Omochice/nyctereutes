package activity_test

import (
	"strings"
	"testing"
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
