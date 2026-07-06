package nyctereutes

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
)

func run(args []string) (exit int, stderr string) {
	exit, _, stderr = runOut(args)
	return exit, stderr
}

func runOut(args []string) (exit int, stdout, stderr string) {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	exit = MainCommand(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	})
	return exit, outBuf.String(), errBuf.String()
}

func TestVersionReportsVersion(t *testing.T) {
	for _, args := range [][]string{{"-v"}, {"--version"}, {"version"}} {
		exit, stdout, stderr := runOut(args)

		if exit != 0 {
			t.Errorf("%v: want exit status 0, got %d (stderr=%q)", args, exit, stderr)
		}
		if strings.TrimSpace(stdout) != version {
			t.Errorf("%v: want stdout %q, got %q", args, version, stdout)
		}
	}
}

func TestInfraRequiresSubcommand(t *testing.T) {
	exit, stderr := run([]string{"infra"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if !strings.Contains(stderr, "import") {
		t.Errorf("want stderr to list the import subcommand, got %q", stderr)
	}
}

func TestHelpIsNotImplemented(t *testing.T) {
	exit, stderr := run([]string{"help"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if !strings.Contains(stderr, "not implemented") {
		t.Errorf("want stderr to contain %q, got %q", "not implemented", stderr)
	}
}

func TestNoSubcommandReportsError(t *testing.T) {
	exit, stderr := run([]string{})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if stderr == "" {
		t.Error("want a usage error on stderr, got empty output")
	}
}

func TestUnknownSubcommandReportsError(t *testing.T) {
	exit, stderr := run([]string{"nope"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if stderr == "" {
		t.Error("want a usage error on stderr, got empty output")
	}
}
