package doc_test

import (
	"strings"
	"testing"
)

func TestDocRequiresSubcommand(t *testing.T) {
	exit, _, stderr := run("doc")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 when no subcommand is given", exit)
	}
	for _, want := range []string{"list", "show"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not offer the %s subcommand\n%s", want, stderr)
		}
	}
}
