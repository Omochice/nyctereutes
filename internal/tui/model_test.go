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

// press feeds each key string through Update in order and returns the model.
func press(m Model, keys ...string) Model {
	for _, k := range keys {
		next, _ := m.Update(key(k))
		m = next.(Model)
	}
	return m
}

func TestInitialCursorOnFirstMR(t *testing.T) {
	m := New(&fakeClient{}, sampleMRs())
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
}

func TestCursorMovesDownAndStopsAtEnd(t *testing.T) {
	m := New(&fakeClient{}, sampleMRs()) // 3 MRs

	m = press(m, "j")
	if m.cursor != 1 {
		t.Fatalf("after j cursor = %d, want 1", m.cursor)
	}

	m = press(m, "j", "j", "j") // past the end
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped at last MR)", m.cursor)
	}
}

func TestCursorMovesUpAndStopsAtTop(t *testing.T) {
	m := New(&fakeClient{}, sampleMRs())
	m = press(m, "j", "j") // cursor at 2

	m = press(m, "k")
	if m.cursor != 1 {
		t.Fatalf("after k cursor = %d, want 1", m.cursor)
	}

	m = press(m, "k", "k", "k") // past the top
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at first MR)", m.cursor)
	}
}

func selectedIIDs(m Model) []int {
	var iids []int
	for _, mr := range m.selectedMRs() {
		iids = append(iids, mr.IID)
	}
	return iids
}

func TestSpaceAndEnterToggleSelection(t *testing.T) {
	for _, k := range []string{"space", "enter"} {
		t.Run(k, func(t *testing.T) {
			m := New(&fakeClient{}, sampleMRs())

			m = press(m, k) // select MR under cursor (IID 12)
			if got := selectedIIDs(m); len(got) != 1 || got[0] != 12 {
				t.Fatalf("after %s selected = %v, want [12]", k, got)
			}

			m = press(m, k) // toggle off
			if got := selectedIIDs(m); len(got) != 0 {
				t.Errorf("after second %s selected = %v, want none", k, got)
			}
		})
	}
}

func TestSelectAllAndDeselectAll(t *testing.T) {
	m := New(&fakeClient{}, sampleMRs())

	m = press(m, "a")
	if got := len(selectedIIDs(m)); got != 3 {
		t.Fatalf("after a selected count = %d, want 3", got)
	}

	m = press(m, "d")
	if got := len(selectedIIDs(m)); got != 0 {
		t.Errorf("after d selected count = %d, want 0", got)
	}
}
