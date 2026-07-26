package dep_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDepListRendersTable(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "list")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	for _, want := range []string{"PROJECT", "MR", "TITLE", "g/proj", "!12", "Bump lodash"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("list output missing %q\n%s", want, stdout)
		}
	}
}

func TestDepListGroup(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "list", "--group")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "GROUP") || !strings.Contains(stdout, "lodash@2.0.0") {
		t.Errorf("group output missing GROUP/key\n%s", stdout)
	}
}

func TestDepListJSON(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "list", "--json")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("list --json is not valid JSON: %v\n%s", err, stdout)
	}
}

func TestDepListEmpty(t *testing.T) {
	fake := &fakeGlab{listJSON: `[]`, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "list")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "No dependency MRs found") {
		t.Errorf("want empty message, got %q", stdout)
	}
}

func TestDepListEmptyJSON(t *testing.T) {
	fake := &fakeGlab{listJSON: `[]`, detailJSON: `{}`}
	exit, stdout, _ := runWithRunner(fake, "dep", "list", "--json")
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if strings.Contains(stdout, "No dependency MRs found") {
		t.Errorf("--json must not emit the plain message, got %q", stdout)
	}
	// Must be an empty array, not null, so consumers can always iterate.
	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Errorf("empty --json = %q, want %q", got, "[]")
	}
}
