package tui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Omochice/nyctereutes/internal/types"
)

var errApprove = errors.New("approve failed")

// fakeClient records the approve/merge calls the model issues so tests can
// assert on them without a real glab.
type fakeClient struct {
	mu          sync.Mutex
	approved    []int
	merged      []int
	mergeMethod []string
	mergeAuto   []bool
	approveFn   func(iid int) error
	mergeFn     func(iid int) error
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

func (f *fakeClient) MergeMR(_ context.Context, _ string, iid int, method string, autoMerge bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.merged = append(f.merged, iid)
	f.mergeMethod = append(f.mergeMethod, method)
	f.mergeAuto = append(f.mergeAuto, autoMerge)
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

func key(name string) tea.KeyPressMsg {
	switch name {
	case keyEnter:
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case keySpace:
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case keyEscape:
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case keyInterrupt:
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		return tea.KeyPressMsg{Code: []rune(name)[0], Text: name}
	}
}

func asModel(updated tea.Model) Model {
	model, _ := updated.(Model)
	return model
}

// press feeds each key name through Update in order and returns the model.
func press(model Model, names ...string) Model {
	for _, name := range names {
		next, _ := model.Update(key(name))
		model = asModel(next)
	}
	return model
}

// typeRunes feeds the runes of text one key press at a time.
func typeRunes(model Model, text string) Model {
	for _, char := range text {
		model = press(model, string(char))
	}
	return model
}

// drain runs cmd and feeds the resulting message(s) back through Update,
// unwrapping tea.Batch so the per-MR commands actually execute.
func drain(model Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return model
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, batched := range msg {
			model = drain(model, batched)
		}
	default:
		next, _ := model.Update(msg)
		model = asModel(next)
	}
	return model
}

func selectedIIDs(model Model) []int {
	selected := model.selectedMRs()
	iids := make([]int, 0, len(selected))
	for _, mergeRequest := range selected {
		iids = append(iids, mergeRequest.IID)
	}
	return iids
}

func visibleIIDs(model Model) []int {
	visible := model.visible()
	iids := make([]int, 0, len(visible))
	for _, mergeRequest := range visible {
		iids = append(iids, mergeRequest.IID)
	}
	return iids
}

func TestInitialCursorOnFirstMR(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	if model.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", model.cursor)
	}
}

func TestCursorMovesDownAndStopsAtEnd(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs()) // 3 MRs

	model = press(model, keyDown)
	if model.cursor != 1 {
		t.Fatalf("after j cursor = %d, want 1", model.cursor)
	}

	model = press(model, keyDown, keyDown, keyDown) // past the end
	if model.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped at last MR)", model.cursor)
	}
}

func TestCursorMovesUpAndStopsAtTop(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keyDown, keyDown) // cursor at 2

	model = press(model, keyUp)
	if model.cursor != 1 {
		t.Fatalf("after k cursor = %d, want 1", model.cursor)
	}

	model = press(model, keyUp, keyUp, keyUp) // past the top
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at first MR)", model.cursor)
	}
}

func TestSpaceAndEnterToggleSelection(t *testing.T) {
	for _, keyName := range []string{keySpace, keyEnter} {
		t.Run(keyName, func(t *testing.T) {
			model := New(&fakeClient{}, sampleMRs())

			model = press(model, keyName) // select MR under cursor (IID 12)
			if got := selectedIIDs(model); len(got) != 1 || got[0] != 12 {
				t.Fatalf("after %s selected = %v, want [12]", keyName, got)
			}

			model = press(model, keyName) // toggle off
			if got := selectedIIDs(model); len(got) != 0 {
				t.Errorf("after second %s selected = %v, want none", keyName, got)
			}
		})
	}
}

func TestSelectAllAndDeselectAll(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())

	model = press(model, keySelectAll)
	if got := len(selectedIIDs(model)); got != 3 {
		t.Fatalf("after a selected count = %d, want 3", got)
	}

	model = press(model, keyClear)
	if got := len(selectedIIDs(model)); got != 0 {
		t.Errorf("after d selected count = %d, want 0", got)
	}
}

func TestListViewRendersRowElements(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySpace) // select the first MR so a checked box renders

	out := model.View().Content
	for _, want := range []string{
		">",           // cursor on the first row
		"[x]",         // checkbox of the selected MR
		"g/a",         // shortened project path of group/a
		"!12",         // MR IID
		"Bump lodash", // title
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q\n%s", want, out)
		}
	}
	// The second MR is unselected, so its box is empty.
	if !strings.Contains(out, "[ ]") {
		t.Errorf("view missing an empty checkbox\n%s", out)
	}
}

func TestListViewMarksUnmergeable(t *testing.T) {
	mrs := []types.MR{
		{IID: 20, Project: "group/x", Title: "Bump foo from 1 to 2", UnmergeableReason: types.ReasonConflict},
	}
	model := New(&fakeClient{}, mrs)
	if !strings.Contains(model.View().Content, "⚠") {
		t.Errorf("unmergeable MR should show a warning marker\n%s", model.View().Content)
	}
}

func TestSlashEntersSearchMode(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySearch)
	if !strings.Contains(model.View().Content, "search:") {
		t.Errorf("search mode should show a search prompt\n%s", model.View().Content)
	}
}

func TestSearchFiltersOnEnter(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySearch)
	model = typeRunes(model, "axios")
	// Not applied until enter is pressed.
	if got := len(visibleIIDs(model)); got != 3 {
		t.Fatalf("before enter visible = %d, want 3 (live filtering disabled)", got)
	}
	model = press(model, keyEnter)
	if got := visibleIIDs(model); len(got) != 1 || got[0] != 13 {
		t.Errorf("after enter visible = %v, want [13]", got)
	}
}

func TestSearchIsCaseInsensitiveAcrossFields(t *testing.T) {
	cases := map[string]struct {
		query string
		want  int // expected single matching IID
	}{
		"title":       {query: "LODASH", want: 12},
		"projectPath": {query: "GROUP/B", want: 13},
		"iid":         {query: "14", want: 14},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			model := New(&fakeClient{}, sampleMRs())
			model = press(model, keySearch)
			model = typeRunes(model, testCase.query)
			model = press(model, keyEnter)
			if got := visibleIIDs(model); len(got) != 1 || got[0] != testCase.want {
				t.Errorf("query %q visible = %v, want [%d]", testCase.query, got, testCase.want)
			}
		})
	}
}

func TestSearchEscCancelsWithoutFiltering(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySearch)
	model = typeRunes(model, "axios")
	model = press(model, keyEscape)
	if got := len(visibleIIDs(model)); got != 3 {
		t.Errorf("after esc visible = %d, want 3 (filter discarded)", got)
	}
}

func TestApplyingFilterClearsSelection(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySpace) // select first MR
	model = press(model, keySearch)
	model = typeRunes(model, "axios")
	model = press(model, keyEnter)
	if got := len(selectedIIDs(model)); got != 0 {
		t.Errorf("after applying filter selected = %d, want 0", got)
	}
}

func TestEmptyFilterResultKeepsCursorInRange(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keyDown, keyDown) // cursor at 2
	model = press(model, keySearch)
	model = typeRunes(model, "zzz-no-match")
	model = press(model, keyEnter)
	if len(visibleIIDs(model)) != 0 {
		t.Fatalf("expected no matches, got %v", visibleIIDs(model))
	}
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 when the list is empty", model.cursor)
	}
	// View must not panic on an empty list.
	_ = model.View().Content
}

func TestRunWithoutSelectionStaysOnList(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	next, cmd := model.Update(key(keyRun))
	model = asModel(next)
	if model.phase != phaseList {
		t.Errorf("phase = %v, want phaseList when nothing is selected", model.phase)
	}
	if cmd != nil {
		t.Errorf("want no command when nothing is selected")
	}
}

func TestRunWithSelectionEntersExecuting(t *testing.T) {
	fake := &fakeClient{approveFn: func(int) error { return nil }}
	model := New(fake, sampleMRs())
	model = press(model, keySpace) // select first MR
	next, cmd := model.Update(key(keyRun))
	model = asModel(next)
	if model.phase != phaseExecuting {
		t.Errorf("phase = %v, want phaseExecuting", model.phase)
	}
	if cmd == nil {
		t.Errorf("want a command to perform the action")
	}
}

func TestApproveModeRunsApproveOnly(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keySelectAll) // select all 3
	next, cmd := model.Update(key(keyRun))
	model = drain(asModel(next), cmd)

	if len(fake.approved) != 3 {
		t.Errorf("approved %d MRs, want 3", len(fake.approved))
	}
	if len(fake.merged) != 0 {
		t.Errorf("merged %d MRs, want 0 in approve mode", len(fake.merged))
	}
	if model.phase != phaseComplete {
		t.Errorf("phase = %v, want phaseComplete after execution", model.phase)
	}
}

func TestMergeModeMergesWithSquashAutoMerge(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keyMode)  // mode -> merge
	model = press(model, keySpace) // select first MR
	next, cmd := model.Update(key(keyRun))
	drain(asModel(next), cmd)

	if len(fake.approved) != 0 {
		t.Errorf("approved %d MRs, want 0 in merge mode", len(fake.approved))
	}
	if len(fake.merged) != 1 {
		t.Fatalf("merged %d MRs, want 1", len(fake.merged))
	}
	if fake.mergeMethod[0] != mergeMethod || !fake.mergeAuto[0] {
		t.Errorf("merge called with method=%q auto=%v, want squash/true", fake.mergeMethod[0], fake.mergeAuto[0])
	}
}

func TestApproveMergeApprovesThenMerges(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keyMode, keyMode) // mode -> approve & merge
	model = press(model, keySpace)         // select first MR (IID 12)
	next, cmd := model.Update(key(keyRun))
	drain(asModel(next), cmd)

	if len(fake.approved) != 1 || fake.approved[0] != 12 {
		t.Errorf("approved = %v, want [12]", fake.approved)
	}
	if len(fake.merged) != 1 || fake.merged[0] != 12 {
		t.Errorf("merged = %v, want [12]", fake.merged)
	}
}

func TestApproveMergeSkipsMergeWhenApproveFails(t *testing.T) {
	fake := &fakeClient{approveFn: func(int) error { return errApprove }}
	model := New(fake, sampleMRs())
	model = press(model, keyMode, keyMode) // mode -> approve & merge
	model = press(model, keySpace)
	next, cmd := model.Update(key(keyRun))
	drain(asModel(next), cmd)

	if len(fake.approved) != 1 {
		t.Errorf("approved %d, want 1 attempt", len(fake.approved))
	}
	if len(fake.merged) != 0 {
		t.Errorf("merged %d, want 0 when approve failed", len(fake.merged))
	}
}

func TestCompleteViewShowsResults(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keySpace) // select IID 12
	next, cmd := model.Update(key(keyRun))
	model = drain(asModel(next), cmd)

	out := model.View().Content
	if !strings.Contains(out, "!12") {
		t.Errorf("complete view should report the processed MR\n%s", out)
	}
}

func TestHelpToggles(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())

	model = press(model, keyHelp)
	if !strings.Contains(model.View().Content, "Keybindings") {
		t.Fatalf("? should show the help view\n%s", model.View().Content)
	}

	model = press(model, keyHelp)
	if strings.Contains(model.View().Content, "Keybindings") {
		t.Errorf("second ? should return to the list view\n%s", model.View().Content)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, keyName := range []string{keyQuit, keyInterrupt} {
		t.Run(keyName, func(t *testing.T) {
			model := New(&fakeClient{}, sampleMRs())
			_, cmd := model.Update(key(keyName))
			if cmd == nil {
				t.Fatalf("%s should return a command", keyName)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("%s should return tea.Quit", keyName)
			}
		})
	}
}

func TestModeStartsAtApproveAndCycles(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	if got := model.modeLabel(); got != labelApprove {
		t.Fatalf("initial mode = %q, want approve", got)
	}

	want := []string{labelMerge, labelApproveMerge, labelApprove}
	for _, wantLabel := range want {
		model = press(model, keyMode)
		if got := model.modeLabel(); got != wantLabel {
			t.Errorf("after m mode = %q, want %q", got, wantLabel)
		}
	}
}
