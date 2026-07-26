package dep_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/dep/tui"
	"github.com/Omochice/nyctereutes/nyctereutes/dep"
)

func TestDepNoSubcommandLaunchesTUIWithSearchResults(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR, detailJSON: `{}`}
	outBuf := &bytes.Buffer{}
	cmd := dep.New(&cli.ProcInout{Stdin: strings.NewReader(""), Stdout: outBuf, Stderr: outBuf}, fake)

	var launched *tui.Model
	dep.SetLaunch(cmd, func(m tui.Model) error {
		launched = &m
		return nil
	})

	if err := cmd.Execute(nil); err != nil {
		t.Fatalf("Execute returned %v", err)
	}
	if launched == nil {
		t.Fatalf("TUI was not launched for a non-empty search")
	}
	if got := len(launched.MRs()); got != 1 {
		t.Errorf("model built with %d MRs, want 1", got)
	}
}

func TestDepNoSubcommandEmptyDoesNotLaunch(t *testing.T) {
	fake := &fakeGlab{listJSON: `[]`, detailJSON: `{}`}
	outBuf := &bytes.Buffer{}
	cmd := dep.New(&cli.ProcInout{Stdin: strings.NewReader(""), Stdout: outBuf, Stderr: outBuf}, fake)

	launched := false
	dep.SetLaunch(cmd, func(tui.Model) error {
		launched = true
		return nil
	})

	if err := cmd.Execute(nil); err != nil {
		t.Fatalf("Execute returned %v", err)
	}
	if launched {
		t.Error("TUI must not launch when no MRs are found")
	}
	if !strings.Contains(outBuf.String(), "No dependency MRs found") {
		t.Errorf("want empty message, got %q", outBuf.String())
	}
}
