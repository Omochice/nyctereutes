package dep

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/dep/tui"
)

var detailPath = regexp.MustCompile(`merge_requests/\d+$`)

// fakeGlab answers the config read with defaults and every api call with the
// scripted list, which is all the search behind the TUI launch needs.
type fakeGlab struct {
	listJSON string
}

func (fake *fakeGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if args[0] != "api" {
		return nil, nil // unset config -> defaults apply
	}
	if detailPath.MatchString(args[len(args)-1]) {
		return []byte(`{}`), nil
	}
	return []byte(fake.listJSON), nil
}

const oneMR = `[{"iid":12,"project_id":7,"title":"Bump lodash from 1.0.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12"}]`

func TestDepNoSubcommandLaunchesTUIWithSearchResults(t *testing.T) {
	fake := &fakeGlab{listJSON: oneMR}
	outBuf := &bytes.Buffer{}
	cmd := New(&cli.ProcInout{Stdin: strings.NewReader(""), Stdout: outBuf, Stderr: outBuf}, fake)

	var launched *tui.Model
	cmd.launch = func(m tui.Model) error {
		launched = &m
		return nil
	}

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
	fake := &fakeGlab{listJSON: `[]`}
	outBuf := &bytes.Buffer{}
	cmd := New(&cli.ProcInout{Stdin: strings.NewReader(""), Stdout: outBuf, Stderr: outBuf}, fake)

	launched := false
	cmd.launch = func(tui.Model) error {
		launched = true
		return nil
	}

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
