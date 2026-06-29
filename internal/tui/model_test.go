package tui

import (
	"context"
	"errors"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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
	f.approved = append(f.approved, iid)
	fn := f.approveFn
	f.mu.Unlock()
	// Call the hook outside the lock so a blocking hook exercises the model's
	// concurrency limit rather than serializing on this mutex.
	if fn != nil {
		return fn(iid)
	}
	return nil
}

func (f *fakeClient) MergeMR(_ context.Context, _ string, iid int, method string, autoMerge bool) error {
	f.mu.Lock()
	f.merged = append(f.merged, iid)
	f.mergeMethod = append(f.mergeMethod, method)
	f.mergeAuto = append(f.mergeAuto, autoMerge)
	fn := f.mergeFn
	f.mu.Unlock()
	if fn != nil {
		return fn(iid)
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

func TestToggleOnEmptyListSelectsNothing(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySearch)
	model = typeRunes(model, "zzz-no-match")
	model = press(model, keyEnter) // filter to an empty list

	model = press(model, keySpace) // must not select a hidden MR

	model.filter = "" // clear the filter to reveal any stray selection
	model.filtered = model.mrs
	if got := len(selectedIIDs(model)); got != 0 {
		t.Errorf("toggle on an empty list selected %d MRs, want 0", got)
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestInterruptQuitsFromSearchMode(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySearch)
	if _, cmd := model.Update(key(keyInterrupt)); !isQuit(cmd) {
		t.Error("ctrl+c should quit even while searching")
	}
}

func TestKeysDoNotRerunDuringExecution(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keySpace) // select first MR
	next, _ := model.Update(key(keyRun))
	model = asModel(next) // now phaseExecuting, action in flight
	if model.phase != phaseExecuting {
		t.Fatalf("phase = %v, want phaseExecuting", model.phase)
	}

	next2, cmd := model.Update(key(keyRun)) // pressing x again must not re-run
	model = asModel(next2)
	if cmd != nil {
		t.Error("x during execution must not start another run")
	}
	if model.phase != phaseExecuting {
		t.Errorf("phase = %v, want phaseExecuting (unchanged)", model.phase)
	}
}

func TestKeysIgnoredOnCompleteScreenExceptQuit(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keySpace)
	next, cmd := model.Update(key(keyRun))
	model = drain(asModel(next), cmd) // phaseComplete

	next2, cmd2 := model.Update(key(keyRun)) // must not re-run actions
	if cmd2 != nil {
		t.Error("x on the complete screen must not run actions")
	}
	if len(fake.approved) != 1 {
		t.Errorf("approved %d times, want 1 (no re-run)", len(fake.approved))
	}
	if _, cmd := asModel(next2).Update(key(keyQuit)); !isQuit(cmd) {
		t.Error("q should quit from the complete screen")
	}
}

func TestExecutionCapsConcurrentActions(t *testing.T) {
	total := maxConcurrentActions + 4
	mrs := make([]types.MR, total)
	for index := range mrs {
		mrs[index] = types.MR{IID: index + 1, Project: "g/p"}
	}

	entered := make(chan int, total)
	release := make(chan struct{})
	fake := &fakeClient{approveFn: func(iid int) error {
		entered <- iid
		<-release
		return nil
	}}

	model := New(fake, mrs)
	model = press(model, keySelectAll)
	_, cmd := model.Update(key(keyRun))
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a batch of per-MR commands")
	}
	for _, command := range batch {
		go command()
	}

	// The cap many actions reach glab and park on release.
	for range maxConcurrentActions {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the capped actions to start")
		}
	}
	// No further action may enter glab while the cap is held by the parked ones.
	for range 100 {
		runtime.Gosched()
		select {
		case extra := <-entered:
			t.Fatalf("more than %d actions ran concurrently (extra !%d)", maxConcurrentActions, extra)
		default:
		}
	}
	close(release)
}

func TestCompleteScreenReturnsToListOnEnter(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keySpace) // select IID 12
	next, cmd := model.Update(key(keyRun))
	model = drain(asModel(next), cmd) // phaseComplete
	if model.phase != phaseComplete {
		t.Fatalf("setup: phase = %v, want phaseComplete", model.phase)
	}

	model = press(model, keyEnter) // return to the list
	if model.phase != phaseList {
		t.Errorf("phase = %v, want phaseList after enter", model.phase)
	}
	if got := len(selectedIIDs(model)); got != 0 {
		t.Errorf("selection should be cleared on return, got %d", got)
	}
	if !strings.Contains(model.View().Content, "mode:") {
		t.Errorf("expected the list view after returning\n%s", model.View().Content)
	}
}

func TestHelpOverlayIgnoresNavigation(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keyDown) // cursor at 1
	model = press(model, keyHelp) // open help
	model = press(model, keyDown) // must be ignored while help is shown
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (navigation ignored under help)", model.cursor)
	}
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

func TestMergeModeMergesWithSquashImmediate(t *testing.T) {
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
	// require-checks defaults off, so merge is immediate rather than auto-merge.
	if fake.mergeMethod[0] != defaultMergeMethod || fake.mergeAuto[0] {
		t.Errorf("merge called with method=%q auto=%v, want squash/false", fake.mergeMethod[0], fake.mergeAuto[0])
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

func TestMergeMethodStartsAtSquashAndCycles(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	if model.method != defaultMergeMethod {
		t.Fatalf("initial merge method = %q, want squash", model.method)
	}

	want := []string{methodMerge, methodRebase, methodSquash}
	for _, wantMethod := range want {
		model = press(model, keyMergeMethod)
		if model.method != wantMethod {
			t.Errorf("after M merge method = %q, want %q", model.method, wantMethod)
		}
	}
}

func TestMergeRunsWithSelectedMethod(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keyMode)        // mode -> merge
	model = press(model, keyMergeMethod) // method squash -> merge
	model = press(model, keySpace)       // select first MR
	next, cmd := model.Update(key(keyRun))
	drain(asModel(next), cmd)

	if len(fake.mergeMethod) != 1 || fake.mergeMethod[0] != methodMerge {
		t.Errorf("merge methods = %v, want [merge]", fake.mergeMethod)
	}
}

func TestListViewShowsMergeMethod(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	if !strings.Contains(model.View().Content, defaultMergeMethod) {
		t.Fatalf("list view should show the default merge method\n%s", model.View().Content)
	}

	model = press(model, keyMergeMethod) // squash -> merge
	if !strings.Contains(model.View().Content, methodMerge) {
		t.Errorf("list view should show the cycled merge method\n%s", model.View().Content)
	}
}

func TestRequireChecksDefaultsOffShowingAllMRs(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	if model.requireChecks {
		t.Fatalf("require-checks should default to off")
	}
	if got := visibleIIDs(model); len(got) != 3 {
		t.Errorf("visible = %v, want all 3 MRs when require-checks is off", got)
	}
}

func TestRequireChecksTogglesOnAndOff(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())

	model = press(model, keyRequireChecks)
	if !model.requireChecks {
		t.Fatalf("c should turn require-checks on")
	}

	model = press(model, keyRequireChecks)
	if model.requireChecks {
		t.Errorf("a second c should turn require-checks off")
	}
}

func TestRequireChecksOnShowsOnlyCISuccess(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs()) // IID 12 success, 13 failure, 14 pending
	model = press(model, keyRequireChecks)
	if got := visibleIIDs(model); len(got) != 1 || got[0] != 12 {
		t.Errorf("visible = %v, want only the CI-success MR [12]", got)
	}
}

func TestRequireChecksOnMergesWithAutoMerge(t *testing.T) {
	fake := &fakeClient{}
	model := New(fake, sampleMRs())
	model = press(model, keyRequireChecks) // require-checks on -> visible only IID 12
	model = press(model, keyMode)          // mode -> merge
	model = press(model, keySpace)         // select the only visible MR (IID 12)
	next, cmd := model.Update(key(keyRun))
	drain(asModel(next), cmd)

	if len(fake.mergeAuto) != 1 || !fake.mergeAuto[0] {
		t.Errorf("merge auto-merge flags = %v, want [true] with require-checks on", fake.mergeAuto)
	}
}

func TestRequireChecksToggleClearsSelection(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySelectAll) // select all visible
	if len(selectedIIDs(model)) == 0 {
		t.Fatalf("precondition: expected a selection before toggling")
	}

	model = press(model, keyRequireChecks)
	if got := selectedIIDs(model); len(got) != 0 {
		t.Errorf("selection = %v, want cleared after toggling require-checks", got)
	}
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after toggling require-checks", model.cursor)
	}
}

func TestRequireChecksFilterComposesWithSearch(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	model = press(model, keySearch)
	model = typeRunes(model, "Bump") // matches IID 12 (success) and 13 (failure)
	model = press(model, keyEnter)
	model = press(model, keyRequireChecks) // narrow to CI success

	if got := visibleIIDs(model); len(got) != 1 || got[0] != 12 {
		t.Errorf("visible = %v, want [12] (search and CI filter combined)", got)
	}
}

func TestListViewShowsRequireChecksState(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	if !strings.Contains(model.View().Content, "require-checks: off") {
		t.Fatalf("list view should show require-checks off by default\n%s", model.View().Content)
	}

	model = press(model, keyRequireChecks)
	if !strings.Contains(model.View().Content, "require-checks: on") {
		t.Errorf("list view should show require-checks on after toggling\n%s", model.View().Content)
	}
}

// ansiEscape is the prefix every lipgloss color sequence starts with; its
// presence marks a string as colored.
const ansiEscape = "\x1b["

func TestCISuccessGlyphIsColored(t *testing.T) {
	out := styledCISymbol(ciStatusSuccess)
	if !strings.Contains(out, ansiEscape) {
		t.Errorf("success glyph should be colored, got %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("colored glyph should still contain the plain glyph, got %q", out)
	}
}

func TestCIStatusesUseDistinctColors(t *testing.T) {
	// Compare only the ANSI color sequences, not the whole rendered string: the
	// glyphs already differ, so full-string comparison would pass even if the
	// colors were identical or missing. The SGR sequences are extracted generically
	// so the assertion does not depend on the implementation's color values.
	sgr := regexp.MustCompile("\x1b\\[[0-9;]*m")
	colorSeq := func(status string) string {
		return strings.Join(sgr.FindAllString(styledCISymbol(status), -1), "")
	}
	success := colorSeq(ciStatusSuccess)
	failure := colorSeq(ciStatusFailure)
	pending := colorSeq(ciStatusPending)
	if success == "" || failure == "" || pending == "" {
		t.Fatalf("each status should emit a color sequence: %q %q %q", success, failure, pending)
	}
	if success == failure || failure == pending || success == pending {
		t.Errorf("CI statuses should use distinct colors: %q %q %q", success, failure, pending)
	}
}

func TestUnknownCIGlyphIsDimmed(t *testing.T) {
	out := styledCISymbol("")
	if !strings.Contains(out, ansiEscape) {
		t.Errorf("unknown status glyph should be dimmed gray, got %q", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("unknown status should render the dash marker, got %q", out)
	}
}

func TestUnmergeableMarkerIsColored(t *testing.T) {
	if !strings.Contains(styledWarn(), ansiEscape) {
		t.Errorf("warning marker should be colored, got %q", styledWarn())
	}
}

func TestMergeableRowHasNoWarningMarker(t *testing.T) {
	mrs := []types.MR{
		{IID: 1, Project: "group/x", Title: "Bump foo from 1 to 2", CIStatus: ciStatusSuccess},
	}
	model := New(&fakeClient{}, mrs)
	if strings.Contains(model.View().Content, "⚠") {
		t.Errorf("mergeable MR should not show a warning marker\n%s", model.View().Content)
	}
}
