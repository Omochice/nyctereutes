package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Omochice/nyctereutes/internal/dep/types"
)

func frameHeight(content string) int {
	return strings.Count(content, "\n") + 1
}

func TestFitHeightCutsTallFrameToHeight(t *testing.T) {
	content := "a\nb\nc\nd\ne\n"

	got := fitHeight(content, 3)

	if frameHeight(got) != 3 {
		t.Errorf("frame height = %d, want 3\n%q", frameHeight(got), got)
	}
	if got != "a\nb\nc" {
		t.Errorf("fitHeight kept %q, want the top lines %q", got, "a\nb\nc")
	}
}

func TestFitHeightKeepsFittingFrameUnchanged(t *testing.T) {
	content := "a\nb\nc\n"

	if got := fitHeight(content, 4); got != content {
		t.Errorf("fitHeight(%q, 4) = %q, want it unchanged", content, got)
	}
}

func TestFitHeightKeepsFrameUnchangedWhenHeightUnknown(t *testing.T) {
	content := "a\nb\nc\nd\ne\n"

	if got := fitHeight(content, 0); got != content {
		t.Errorf("fitHeight(%q, 0) = %q, want it unchanged", content, got)
	}
}

func manyMRs(count int) []types.MR {
	mrs := make([]types.MR, 0, count)
	for index := range count {
		mrs = append(mrs, types.MR{
			IID:      index + 1,
			Project:  "group/p",
			Title:    fmt.Sprintf("Bump dep%d", index),
			CIStatus: "success",
		})
	}
	return mrs
}

func resize(model Model, height int) Model {
	next, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: height})
	return asModel(next)
}

func TestListViewFitsTerminalHeight(t *testing.T) {
	model := resize(New(&fakeClient{}, manyMRs(40)), 16)

	view := model.View().Content
	if frameHeight(view) > 16 {
		t.Errorf("list frame height = %d, want at most 16\n%s", frameHeight(view), view)
	}
}

func TestHelpViewFitsTerminalShorterThanHelpText(t *testing.T) {
	const height = 8
	if frameHeight(helpText) <= height {
		t.Fatalf("help text height %d must exceed %d for this test", frameHeight(helpText), height)
	}
	model := press(resize(New(&fakeClient{}, sampleMRs()), height), keyHelp)

	view := model.View().Content
	if frameHeight(view) > height {
		t.Errorf("help frame height = %d, want at most %d\n%s", frameHeight(view), height, view)
	}
}
