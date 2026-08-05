package doc_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/nyctereutes"
)

var errGlabUnused = errors.New("doc answered from something other than the embedded documents")

// Every document the doc command serves is compiled into the binary, so a glab
// invocation means the command reached for GitLab instead; the fake fails the
// test loudly rather than answering.
type fakeDocGlab struct{}

func (fakeDocGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	return nil, fmt.Errorf("%w: %v", errGlabUnused, args)
}

// Drives the whole command tree, because the exit code and the diagnostics
// these tests assert on are produced by the dispatcher rather than by Execute.
func run(args ...string) (exit int, stdout, stderr string) {
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	exit = nyctereutes.Dispatch(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	}, fakeDocGlab{})
	return exit, outBuf.String(), errBuf.String()
}
