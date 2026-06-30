package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunRejectsNonInteractiveStreams(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	// In-memory streams are not terminals, standing in for redirected or piped
	// stdio; Run must refuse rather than start the TUI against them.
	err := Run(model, strings.NewReader(""), &bytes.Buffer{})
	if !errors.Is(err, errNotInteractive) {
		t.Errorf("Run on non-terminal streams = %v, want errNotInteractive", err)
	}
}
