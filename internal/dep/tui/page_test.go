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
