package nyctereutes

import (
	"bytes"
	"context"
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

// Answers an MR search with an empty list. The help tests need a runner only
// so the tree can be built; reaching it at all would mean help executed the
// target command, which is exactly what they assert never happens.
type fakeHelpGlab struct{}

func (fakeHelpGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if args[0] == "api" {
		return []byte(`[]`), nil
	}
	return nil, nil
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

func TestHelpMatchesHelpFlag(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		flagArgs []string
		helpArgs []string
	}{
		{"top-level usage", []string{"--help"}, []string{"help"}},
		{"subcommand usage", []string{"dep", "--help"}, []string{"help", "dep"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			flagExit, wantUsage, _ := runOut(testCase.flagArgs)
			if flagExit != 0 || wantUsage == "" {
				t.Fatalf("%v must supply the reference usage text, got exit %d stdout %q", testCase.flagArgs, flagExit, wantUsage)
			}

			exit, stdout, stderr := runOut(testCase.helpArgs)

			if exit != 0 {
				t.Errorf("want exit status 0, got %d (stderr=%q)", exit, stderr)
			}
			if stdout != wantUsage {
				t.Errorf("want the same usage text as %v %q, got %q", testCase.flagArgs, wantUsage, stdout)
			}
			if stderr != "" {
				t.Errorf("want empty stderr, got %q", stderr)
			}
		})
	}
}

func TestHelpNeverExecutesTheTargetCommand(t *testing.T) {
	// The fake runner is local to this test: the command packages have their
	// own harness for driving the tree, and only the help path needs an
	// injected runner here.
	runFake := func(args ...string) (exit int, stdout, stderr string) {
		outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
		exit = Dispatch(args, &cli.ProcInout{
			Stdin:  strings.NewReader(""),
			Stdout: outBuf,
			Stderr: errBuf,
		}, fakeHelpGlab{})
		return exit, outBuf.String(), errBuf.String()
	}

	refExit, wantUsage, _ := runFake("dep", "list", "--help")
	if refExit != 0 || wantUsage == "" {
		t.Fatalf("dep list --help must supply the reference usage text, got exit %d stdout %q", refExit, wantUsage)
	}

	// The outer parser consumes the first "--", so one terminator survives
	// into the help command's arguments.
	exit, stdout, stderr := runFake("help", "dep", "list", "--", "--")

	if exit != 0 {
		t.Errorf("want exit status 0, got %d (stderr=%q)", exit, stderr)
	}
	if stdout != wantUsage {
		t.Errorf("want the usage text of dep list --help %q, got %q", wantUsage, stdout)
	}
	if stderr != "" {
		t.Errorf("want empty stderr, got %q", stderr)
	}
}

func TestHelpWithUnknownSubcommandReportsError(t *testing.T) {
	exit, stdout, stderr := runOut([]string{"help", "nope"})

	if exit != 1 {
		t.Errorf("want exit status 1, got %d", exit)
	}
	if stdout != "" {
		t.Errorf("want no usage on stdout for an unknown command, got %q", stdout)
	}
	if !strings.Contains(stderr, "nope") {
		t.Errorf("want stderr to report the unknown command, got %q", stderr)
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
