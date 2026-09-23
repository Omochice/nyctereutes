package activity_test

import (
	"bytes"
	"context"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/nyctereutes"
)

// Scripts the glab calls the command makes: "api user" answers with a fixed
// account, the username lookup knows only "alice", and the events request
// answers with the scripted events. The requested paths are recorded so a
// test can inspect the query the command built.
type fakeGlab struct {
	events string
	err    error
	paths  []string
}

func (fake *fakeGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	if fake.err != nil {
		return nil, fake.err
	}
	path := args[len(args)-1]
	fake.paths = append(fake.paths, path)
	switch {
	case path == "user":
		return []byte(`{"id":42,"username":"me"}`), nil
	case path == "users?username=alice":
		return []byte(`[{"id":7,"username":"alice"}]`), nil
	case strings.HasPrefix(path, "users?username="):
		return []byte(`[]`), nil
	}
	return []byte(fake.events), nil
}

// Drives the whole command tree with an injected glab runner, so the exit code
// and diagnostics asserted on are the dispatcher's, not Execute's alone.
func runWithRunner(runner glab.Runner, args ...string) (exit int, stdout, stderr string) {
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	exit = nyctereutes.Dispatch(args, &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: outBuf,
		Stderr: errBuf,
	}, runner)
	return exit, outBuf.String(), errBuf.String()
}

const somePushes = `[` +
	`{"action_name":"pushed to","target_type":"Project",` +
	`"push_data":{"commit_count":3,"action":"pushed","ref_type":"branch","ref_count":null}},` +
	`{"action_name":"opened","target_type":"MergeRequest","target_id":444342411}]`
