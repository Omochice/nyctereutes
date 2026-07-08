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
	// "-v help" pairs the flag with a subcommand whose Execute would add the
	// usage text to stdout, proving the flag short-circuits before any
	// subcommand runs.
	for _, args := range [][]string{{"-v"}, {"--version"}, {"version"}, {"-v", "help"}} {
		exit, stdout, stderr := runOut(args)

		if exit != 0 {
			t.Errorf("%v: want exit status 0, got %d (stderr=%q)", args, exit, stderr)
		}
		if strings.TrimSpace(stdout) != version {
			t.Errorf("%v: want stdout %q, got %q", args, version, stdout)
		}
		if stderr != "" {
			t.Errorf("%v: want empty stderr, got %q", args, stderr)
		}
	}
}

func TestVersionFlagDoesNotMaskParseError(t *testing.T) {
	exit, stdout, stderr := runOut([]string{"--version", "--bogus"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if stdout != "" {
		t.Errorf("want no version output on a failed parse, got stdout %q", stdout)
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("want stderr to report the unknown flag, got %q", stderr)
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

func TestHelpPrintsUsage(t *testing.T) {
	helpFlagExit, wantUsage, _ := runOut([]string{"--help"})
	if helpFlagExit != 0 || wantUsage == "" {
		t.Fatalf("--help must supply the reference usage text, got exit %d stdout %q", helpFlagExit, wantUsage)
	}

	exit, stdout, stderr := runOut([]string{"help"})

	if exit != 0 {
		t.Errorf("want exit status 0, got %d (stderr=%q)", exit, stderr)
	}
	if stdout != wantUsage {
		t.Errorf("want the same usage text as --help %q, got %q", wantUsage, stdout)
	}
	if stderr != "" {
		t.Errorf("want empty stderr, got %q", stderr)
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
