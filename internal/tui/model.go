package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Omochice/nyctereutes/internal/types"
)

// Client is the subset of gitlab.Client the TUI drives. It is an interface so
// tests can inject a fake that records calls instead of shelling out to glab.
type Client interface {
	ApproveMR(ctx context.Context, project string, iid int) error
	MergeMR(ctx context.Context, project string, iid int, method string, autoMerge bool) error
}

// mode is the action the user applies to the selected MRs.
type mode int

const (
	modeApprove mode = iota
	modeMerge
	modeApproveMerge
)

func (md mode) label() string {
	switch md {
	case modeMerge:
		return "merge"
	case modeApproveMerge:
		return "approve & merge"
	default:
		return "approve"
	}
}

// Model is the bubbletea model backing the interactive dep view.
type Model struct {
	client Client
	mrs    []types.MR
	cursor int
	// selected holds the indices into mrs that the user has checked.
	selected map[int]bool
	mode     mode
}

// modeLabel names the current action mode for display.
func (m Model) modeLabel() string { return m.mode.label() }

// New builds a Model showing mrs, driving approve/merge through client.
func New(client Client, mrs []types.MR) Model {
	return Model{
		client:   client,
		mrs:      mrs,
		selected: make(map[int]bool),
	}
}

// selectedMRs returns the checked MRs in display order.
func (m Model) selectedMRs() []types.MR {
	var out []types.MR
	for i, mr := range m.mrs {
		if m.selected[i] {
			out = append(out, mr)
		}
	}
	return out
}

// Init implements tea.Model; the MRs are loaded before the program starts, so
// there is no initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "j":
		if m.cursor < len(m.mrs)-1 {
			m.cursor++
		}
	case "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "space", "enter":
		m.selected[m.cursor] = !m.selected[m.cursor]
	case "a":
		for i := range m.mrs {
			m.selected[i] = true
		}
	case "d":
		m.selected = make(map[int]bool)
	case "m":
		m.mode = (m.mode + 1) % 3
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() tea.View {
	return tea.NewView(m.renderList())
}

func (m Model) renderList() string {
	var b strings.Builder
	for i, mr := range m.mrs {
		b.WriteString(m.renderRow(i, mr))
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\nmode: %s  (m: change, x: run, ?: help, q: quit)\n", m.modeLabel())
	return b.String()
}

func (m Model) renderRow(index int, mr types.MR) string {
	cursor := " "
	if index == m.cursor {
		cursor = ">"
	}
	checkbox := "[ ]"
	if m.selected[index] {
		checkbox = "[x]"
	}
	warn := " "
	if mr.UnmergeableReason != "" {
		warn = "⚠"
	}
	return fmt.Sprintf("%s %s %s %s %s !%d - %s",
		cursor, checkbox, ciSymbol(mr.CIStatus), warn, pathShorten(mr.Project), mr.IID, mr.Title)
}

// ciSymbol maps a normalized pipeline status to a single-column glyph.
func ciSymbol(status string) string {
	switch status {
	case "success":
		return "✓"
	case "failure":
		return "✗"
	case "pending":
		return "◌"
	default:
		return " "
	}
}
