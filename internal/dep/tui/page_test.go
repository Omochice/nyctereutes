package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/dep/types"
)

func TestPageBounds(t *testing.T) {
	tests := []struct {
		name                    string
		cursor, total, capacity int
		wantStart, wantEnd      int
	}{
		{name: "first page", cursor: 5, total: 40, capacity: 13, wantStart: 0, wantEnd: 13},
		{name: "middle page", cursor: 13, total: 40, capacity: 13, wantStart: 13, wantEnd: 26},
		{name: "last page aligned to the end", cursor: 39, total: 40, capacity: 13, wantStart: 27, wantEnd: 40},
		{name: "total that fits", cursor: 2, total: 3, capacity: 13, wantStart: 0, wantEnd: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end := pageBounds(test.cursor, test.total, test.capacity)
			if start != test.wantStart || end != test.wantEnd {
				t.Errorf("pageBounds(%d, %d, %d) = (%d, %d), want (%d, %d)",
					test.cursor, test.total, test.capacity, start, end, test.wantStart, test.wantEnd)
			}
		})
	}
}

func rowLine(view string, iid string) (string, bool) {
	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, " !"+iid+" - ") {
			return line, true
		}
	}
	return "", false
}

func TestPagedListKeepsCursorAndStatusLineOnScreen(t *testing.T) {
	model := resize(New(&fakeClient{}, manyMRs(40)), 16)

	view := model.View().Content
	if frameHeight(view) > 16 {
		t.Errorf("frame height = %d, want at most 16\n%s", frameHeight(view), view)
	}
	if line, ok := rowLine(view, "1"); !ok || !strings.HasPrefix(line, ">") {
		t.Errorf("cursor row !1 should be shown with the cursor\n%s", view)
	}
	if !strings.Contains(view, "mode:") {
		t.Errorf("status line should be shown\n%s", view)
	}
	if !strings.Contains(view, "rows 1-13 of 40") {
		t.Errorf("separator should show the page position\n%s", view)
	}
}

func TestPagedListFollowsCursorToNextPage(t *testing.T) {
	model := resize(New(&fakeClient{}, manyMRs(40)), 16)
	for range 13 {
		model = press(model, keyDown)
	}

	view := model.View().Content
	if line, ok := rowLine(view, "14"); !ok || !strings.HasPrefix(line, ">") {
		t.Errorf("cursor row !14 should be shown with the cursor\n%s", view)
	}
	if _, ok := rowLine(view, "1"); ok {
		t.Errorf("row !1 should be off the page\n%s", view)
	}
	if !strings.Contains(view, "rows 14-26 of 40") {
		t.Errorf("separator should show the second page\n%s", view)
	}
}

func TestFittingListRendersSameAfterResize(t *testing.T) {
	model := New(&fakeClient{}, sampleMRs())
	before := model.View().Content

	after := resize(model, 16).View().Content
	if after != before {
		t.Errorf("a fitting list changed after resize\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestPagedListKeepsStatusAndErrorOnScreen(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "single line", err: errExternal},
		{name: "multi line", err: errors.New("glab failed\nsecond line\nthird line")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := New(&fakeClient{}, manyMRs(40), WithOpen(func(types.MR) error { return test.err }))
			model = resize(model, 16)
			next, cmd := model.Update(key(keyOpen))
			model = drain(asModel(next), cmd)

			view := model.View().Content
			if frameHeight(view) > 16 {
				t.Errorf("frame height = %d, want at most 16\n%s", frameHeight(view), view)
			}
			if !strings.Contains(view, "mode:") {
				t.Errorf("status line should be shown\n%s", view)
			}
			if !strings.Contains(view, "error: "+test.err.Error()) {
				t.Errorf("the whole error should be shown\n%s", view)
			}
		})
	}
}

func TestSelectionOnLaterPageMarksItsRow(t *testing.T) {
	model := resize(New(&fakeClient{}, manyMRs(40)), 16)
	for range 20 {
		model = press(model, keyDown)
	}
	model = press(model, keySpace)

	view := model.View().Content
	if line, ok := rowLine(view, "21"); !ok || !strings.Contains(line, "[x]") {
		t.Errorf("row !21 should be shown selected\n%s", view)
	}
	if got := selectedIIDs(model); !slices.Equal(got, []int{21}) {
		t.Errorf("selected = %v, want [21]", got)
	}
}

func TestListInTinyTerminalFitsWithoutPanic(t *testing.T) {
	model := resize(New(&fakeClient{}, manyMRs(5)), 2)

	view := model.View().Content
	if frameHeight(view) > 2 {
		t.Errorf("frame height = %d, want at most 2\n%s", frameHeight(view), view)
	}
}

func TestWindowBounds(t *testing.T) {
	tests := []struct {
		name                    string
		offset, total, capacity int
		wantStart, wantEnd      int
	}{
		{name: "offset zero", offset: 0, total: 20, capacity: 6, wantStart: 0, wantEnd: 6},
		{name: "middle offset", offset: 5, total: 20, capacity: 6, wantStart: 5, wantEnd: 11},
		{name: "offset past the end is clamped", offset: 30, total: 20, capacity: 6, wantStart: 14, wantEnd: 20},
		{name: "negative offset is clamped", offset: -1, total: 20, capacity: 6, wantStart: 0, wantEnd: 6},
		{name: "total that fits", offset: 3, total: 3, capacity: 6, wantStart: 0, wantEnd: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end := windowBounds(test.offset, test.total, test.capacity)
			if start != test.wantStart || end != test.wantEnd {
				t.Errorf("windowBounds(%d, %d, %d) = (%d, %d), want (%d, %d)",
					test.offset, test.total, test.capacity, start, end, test.wantStart, test.wantEnd)
			}
		})
	}
}

func runAll(model Model) Model {
	model = press(model, keySelectAll)
	next, cmd := model.Update(key(keyRun))
	return drain(asModel(next), cmd)
}

func completeScreen(client Client, count, height int) Model {
	return resize(runAll(New(client, manyMRs(count))), height)
}

func TestScrollableResultsFitTerminal(t *testing.T) {
	model := completeScreen(&fakeClient{}, 20, 10)

	view := model.View().Content
	if frameHeight(view) > 10 {
		t.Errorf("frame height = %d, want at most 10\n%s", frameHeight(view), view)
	}
	for _, want := range []string{"Done:", "lines 1-6 of 20", "(j/k: scroll, enter: back to list, q: quit)"} {
		if !strings.Contains(view, want) {
			t.Errorf("results view should contain %q\n%s", want, view)
		}
	}
}

func TestResultsScrollWithJAndK(t *testing.T) {
	top := completeScreen(&fakeClient{}, 20, 10)

	down := press(top, keyDown).View().Content
	if _, ok := rowLine(down, "7"); !ok {
		t.Errorf("j should reveal !7\n%s", down)
	}
	if _, ok := rowLine(down, "1"); ok {
		t.Errorf("j should hide !1\n%s", down)
	}
	if back := press(top, keyDown, keyUp).View().Content; back != top.View().Content {
		t.Errorf("k should scroll back to the top\n%s", back)
	}
	if atTop := press(top, keyUp).View().Content; atTop != top.View().Content {
		t.Errorf("k at the top should do nothing\n%s", atTop)
	}

	bottom := top
	for range 14 {
		bottom = press(bottom, keyDown)
	}
	if !strings.Contains(bottom.View().Content, "lines 15-20 of 20") {
		t.Fatalf("14 presses of j should reach the bottom\n%s", bottom.View().Content)
	}
	if past := press(bottom, keyDown).View().Content; past != bottom.View().Content {
		t.Errorf("j at the bottom should do nothing\n%s", past)
	}
}

func TestMultiLineFailureCountsAsSeveralBodyLines(t *testing.T) {
	failing := &fakeClient{approveFn: func(int) error { return errors.New("glab failed\nsecond line\nthird line") }}
	model := completeScreen(failing, 5, 12)

	view := model.View().Content
	if frameHeight(view) > 12 {
		t.Errorf("frame height = %d, want at most 12\n%s", frameHeight(view), view)
	}
	if !strings.Contains(view, "lines 1-8 of 15") {
		t.Errorf("each error line should count as a body line\n%s", view)
	}
}

func TestFittingResultsRenderSameAfterResizeAndJ(t *testing.T) {
	model := runAll(New(&fakeClient{}, sampleMRs()))
	before := model.View().Content

	resized := resize(model, 16)
	if after := resized.View().Content; after != before {
		t.Errorf("fitting results changed after resize\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if scrolled := press(resized, keyDown).View().Content; scrolled != before {
		t.Errorf("j should not change fitting results\n%s", scrolled)
	}
}

func TestResultsStartFromTopAfterNextRun(t *testing.T) {
	model := press(completeScreen(&fakeClient{}, 20, 10), keyDown, keyDown, keyDown, keyEnter)
	model = runAll(model)

	if view := model.View().Content; !strings.Contains(view, "lines 1-6 of 20") {
		t.Errorf("a new run should show its results from the top\n%s", view)
	}
}

func scrolledToBottom(t *testing.T) Model {
	t.Helper()
	model := completeScreen(&fakeClient{}, 20, 10)
	for range 14 {
		model = press(model, keyDown)
	}
	if !strings.Contains(model.View().Content, "lines 15-20 of 20") {
		t.Fatalf("14 presses of j should reach the bottom\n%s", model.View().Content)
	}
	return model
}

func TestResultsWindowStaysFullAfterGrowingPastOffset(t *testing.T) {
	model := resize(scrolledToBottom(t), 16)

	view := model.View().Content
	if frameHeight(view) > 16 {
		t.Errorf("frame height = %d, want at most 16\n%s", frameHeight(view), view)
	}
	if !strings.Contains(view, "lines 9-20 of 20") {
		t.Errorf("a grown terminal should still show a full window\n%s", view)
	}
	if up := press(model, keyUp).View().Content; !strings.Contains(up, "lines 8-19 of 20") {
		t.Errorf("one k after growing should scroll up one line\n%s", up)
	}
}

func TestResultsKeepOffsetAfterShrink(t *testing.T) {
	model := resize(scrolledToBottom(t), 7)

	view := model.View().Content
	if frameHeight(view) > 7 {
		t.Errorf("frame height = %d, want at most 7\n%s", frameHeight(view), view)
	}
	if !strings.Contains(view, "lines 15-17 of 20") {
		t.Errorf("a shrunk terminal should keep the first shown line\n%s", view)
	}
}
