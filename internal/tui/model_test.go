package tui

import (
	"context"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Omochice/nyctereutes/internal/types"
)

// fakeClient records the approve/merge calls the model issues so tests can
// assert on them without a real glab.
type fakeClient struct {
	mu        sync.Mutex
	approved  []int
	merged    []int
	approveFn func(iid int) error
	mergeFn   func(iid int) error
}

func (f *fakeClient) ApproveMR(_ context.Context, _ string, iid int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.approved = append(f.approved, iid)
	if f.approveFn != nil {
		return f.approveFn(iid)
	}
	return nil
}

func (f *fakeClient) MergeMR(_ context.Context, _ string, iid int, _ string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.merged = append(f.merged, iid)
	if f.mergeFn != nil {
		return f.mergeFn(iid)
	}
	return nil
}

func sampleMRs() []types.MR {
	return []types.MR{
		{IID: 12, Project: "group/a", Title: "Bump lodash from 1.0.0 to 2.0.0", CIStatus: "success"},
		{IID: 13, Project: "group/b", Title: "Bump axios from 1.0.0 to 1.1.0", CIStatus: "failure"},
		{IID: 14, Project: "group/c", Title: "Update dependency react to 19.0.0", CIStatus: "pending"},
	}
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
}

func TestInitialCursorOnFirstMR(t *testing.T) {
	m := New(&fakeClient{}, sampleMRs())
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
}
