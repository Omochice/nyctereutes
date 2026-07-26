package color

import (
	"bytes"
	"testing"
)

// A non-file writer such as the buffer used throughout the command tests is
// never a terminal, so color must stay off and captured output plain.
func TestEnabledFalseForNonTerminal(t *testing.T) {
	if Enabled(&bytes.Buffer{}) {
		t.Error("Enabled(*bytes.Buffer) = true, want false for a non-terminal writer")
	}
}

// NO_COLOR disables color by its mere presence, so an empty value counts too.
// This is asserted on disabled directly because Enabled also demands a terminal
// writer, which a test buffer can never be.
func TestDisabledHonorsNoColorPresence(t *testing.T) {
	for _, value := range []string{"", "1"} {
		t.Setenv("NO_COLOR", value)
		if !disabled() {
			t.Errorf("disabled() = false with NO_COLOR=%q present, want true", value)
		}
	}
}
