package nyctereutes

import (
	"context"
	"errors"
	"regexp"
	"sync"
)

var detailPath = regexp.MustCompile(`merge_requests/\d+$`)

var (
	errStub500 = errors.New("500 Internal Server Error")
	errStub409 = errors.New("409 Conflict")
)

// fakeGlab scripts glab responses and records destructive calls.
type fakeGlab struct {
	mu         sync.Mutex
	listJSON   string
	detailJSON string
	approveErr error
	mergeErr   error
	approved   [][]string
	merged     [][]string
}

func (fake *fakeGlab) Run(_ context.Context, args ...string) ([]byte, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()

	switch {
	case args[0] == "config":
		return nil, nil // unset -> defaults apply
	case args[0] == "api":
		path := args[len(args)-1]
		if detailPath.MatchString(path) {
			return []byte(fake.detailJSON), nil
		}
		return []byte(fake.listJSON), nil
	case args[0] == "mr" && args[1] == "approve":
		fake.approved = append(fake.approved, args)
		return nil, fake.approveErr
	case args[0] == "mr" && args[1] == "merge":
		fake.merged = append(fake.merged, args)
		return nil, fake.mergeErr
	}
	return nil, nil
}

const oneMR = `[{"iid":12,"project_id":7,"title":"Bump lodash from 1.0.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12"}]`

const twoMRsSameGroup = `[` +
	`{"iid":12,"project_id":7,"title":"Bump lodash from 1.0.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12"},` +
	`{"iid":13,"project_id":8,"title":"Bump lodash from 1.1.0 to 2.0.0",` +
	`"web_url":"https://gitlab.com/g/other/-/merge_requests/13"}]`
