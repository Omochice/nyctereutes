package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Omochice/nyctereutes/internal/types"
)

// Client is the subset of gitlab.Client the TUI drives. It is an interface so
// tests can inject a fake that records calls instead of shelling out to glab.
type Client interface {
	ApproveMR(ctx context.Context, project string, iid int) error
	MergeMR(ctx context.Context, project string, iid int, method string, autoMerge bool) error
}

// Model is the bubbletea model backing the interactive dep view.
type Model struct {
	client Client
	mrs    []types.MR
	cursor int
}

// New builds a Model showing mrs, driving approve/merge through client.
func New(client Client, mrs []types.MR) Model {
	return Model{
		client: client,
		mrs:    mrs,
	}
}

// Init implements tea.Model; the MRs are loaded before the program starts, so
// there is no initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View implements tea.Model.
func (m Model) View() tea.View { return tea.NewView("") }
