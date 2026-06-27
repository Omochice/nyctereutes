package nyctereutes

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
)

func run(args []string) (exit int, stdout, stderr string) {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	exit = MainCommand(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	})
	return exit, outBuf.String(), errBuf.String()
}

func TestDepIsNotImplemented(t *testing.T) {
	exit, _, stderr := run([]string{"dep"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if !strings.Contains(stderr, "not implemented") {
		t.Errorf("want stderr to contain %q, got %q", "not implemented", stderr)
	}
}

func TestInfraIsNotImplemented(t *testing.T) {
	exit, _, stderr := run([]string{"infra"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if !strings.Contains(stderr, "not implemented") {
		t.Errorf("want stderr to contain %q, got %q", "not implemented", stderr)
	}
}

func TestHelpIsNotImplemented(t *testing.T) {
	exit, _, stderr := run([]string{"help"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if !strings.Contains(stderr, "not implemented") {
		t.Errorf("want stderr to contain %q, got %q", "not implemented", stderr)
	}
}

func TestNoSubcommandReportsError(t *testing.T) {
	exit, _, stderr := run([]string{})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if stderr == "" {
		t.Error("want a usage error on stderr, got empty output")
	}
}

func TestUnknownSubcommandReportsError(t *testing.T) {
	exit, _, stderr := run([]string{"nope"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if stderr == "" {
		t.Error("want a usage error on stderr, got empty output")
	}
}
