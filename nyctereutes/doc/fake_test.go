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

type fakeDocGlab struct{}

func (fakeDocGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	return nil, fmt.Errorf("%w: %v", errGlabUnused, args)
}

func run(args ...string) (exit int, stdout, stderr string) {
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	exit = nyctereutes.Dispatch(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	}, fakeDocGlab{})
	return exit, outBuf.String(), errBuf.String()
}
