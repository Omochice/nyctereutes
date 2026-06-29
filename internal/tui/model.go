package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Omochice/nyctereutes/internal/types"
)

// The subset of gitlab.Client the TUI drives. It is an interface so
// tests can inject a fake that records calls instead of shelling out to glab.
type Client interface {
	ApproveMR(ctx context.Context, project string, iid int) error
	MergeMR(ctx context.Context, project string, iid int, method string, autoMerge bool) error
}

// The action the user applies to the selected MRs.
type mode int

const (
	modeApprove mode = iota
	modeMerge
	modeApproveMerge
)

// The number of modes, used to wrap when cycling. It is a plain
// int (not a mode) so it is not treated as an enum value in exhaustive switches.
const modeCount = 3

const (
	labelApprove      = "approve"
	labelMerge        = "merge"
	labelApproveMerge = "approve & merge"
)

func (md mode) label() string {
	switch md {
	case modeApprove:
		return labelApprove
	case modeMerge:
		return labelMerge
	case modeApproveMerge:
		return labelApproveMerge
	default:
		return labelApprove
	}
}

// Key names as returned by tea.KeyPressMsg.String().
const (
	keyDown      = "j"
	keyUp        = "k"
	keySpace     = "space"
	keyEnter     = "enter"
	keyEscape    = "esc"
	keyBackspace = "backspace"
	keyQuit      = "q"
	keyInterrupt = "ctrl+c"
	keySearch    = "/"
	keyHelp      = "?"
	keySelectAll = "a"
	keyClear     = "d"
	keyMode      = "m"
	keyRun       = "x"
)

// The screen the model is currently showing.
type phase int

const (
	phaseList phase = iota
	phaseExecuting
	phaseComplete
)

// The merge methods cycled through by the M key, in order. The first is the
// default a freshly built model uses.
var mergeMethodCycle = []string{"squash", "merge", "rebase"}

// The merge method a freshly built model starts on.
const defaultMergeMethod = "squash"

// The most glab calls a single run fires at once. tea.Batch would otherwise
// spawn one subprocess per selected MR, so a large selection could overwhelm
// the host and GitLab; this matches internal/gitlab's status-fetch worker cap.
const maxConcurrentActions = 10

// Records the outcome of applying the current mode to one MR.
type actionResult struct {
	mr  types.MR
	err error
}

// Sent back to Update when one MR's action finishes.
type mrResultMsg actionResult

// The bubbletea model backing the interactive dep view.
type Model struct {
	client Client
	mrs    []types.MR
	// The MRs left after the committed filter; recomputed only when the filter
	// changes rather than on every render or keystroke.
	filtered []types.MR
	cursor   int
	// The indices into the filtered MR list the user has checked. They stay
	// valid because changing the filter clears the selection.
	selected map[int]bool
	mode     mode
	// The merge method applied when running merge or approve & merge; cycled by M.
	method string
	// When non-empty, restricts the visible MRs to those matching it.
	filter string
	// True while the user types a query; searchBuf holds the in-progress text
	// that becomes the filter only when committed with enter.
	searching bool
	searchBuf string

	phase   phase
	pending int            // MR actions still in flight during phaseExecuting
	results []actionResult // outcomes shown in phaseComplete
	helping bool           // true while the help overlay is shown
}

// Builds a Model showing mrs, driving approve/merge through client.
func New(client Client, mrs []types.MR) Model {
	return Model{
		client:   client,
		mrs:      mrs,
		filtered: mrs,
		selected: make(map[int]bool),
		method:   defaultMergeMethod,
	}
}

// Returns the merge requests the model was built with.
func (m Model) MRs() []types.MR { return m.mrs }

// Implements tea.Model; the MRs are loaded before the program starts, so
// there is no initial command.
func (m Model) Init() tea.Cmd { return nil }

// Implements tea.Model.
//
//nolint:ireturn // bubbletea's Update must return the tea.Model interface.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case mrResultMsg:
		return m.recordResult(typed), nil
	case tea.KeyPressMsg:
		return m.handleKey(typed)
	default:
		return m, nil
	}
}

// Implements tea.Model. The help overlay and the executing/complete phases
// each own a screen; every other state shows the list.
func (m Model) View() tea.View {
	switch {
	case m.helping:
		return tea.NewView(helpText)
	case m.phase == phaseExecuting:
		return tea.NewView(fmt.Sprintf("Executing %s on %d MR(s)...\n", m.modeLabel(), m.pending))
	case m.phase == phaseComplete:
		return tea.NewView(m.renderResults())
	default:
		return tea.NewView(m.renderList())
	}
}

// Names the current action mode for display.
func (m Model) modeLabel() string { return m.mode.label() }

// Folds one finished MR action into the results and advances to
// the complete screen once every action has reported back.
func (m Model) recordResult(result mrResultMsg) Model {
	m.results = append(m.results, actionResult(result))
	m.pending--
	if m.pending <= 0 {
		m.phase = phaseComplete
	}
	return m
}

// Routes a key press to the active screen so list edits and runs
// happen only on the list itself. ctrl+c quits from anywhere; q quits except
// while searching, where it is an ordinary character; help toggles from the
// list or the help overlay.
//
//nolint:ireturn // bubbletea's Update contract requires the tea.Model interface.
func (m Model) handleKey(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := keyMsg.String()
	if name == keyInterrupt {
		return m, tea.Quit
	}
	if m.searching {
		return m.updateSearch(keyMsg), nil
	}
	if name == keyHelp && (m.helping || m.phase == phaseList) {
		m.helping = !m.helping
		return m, nil
	}
	if m.phase == phaseList && !m.helping {
		return m.updateList(name)
	}
	return m.exitKey(name)
}

// Handles keys on the non-list screens (help, executing, complete): the
// complete screen returns to the list on enter/esc, and q quits from any of
// them.
//
//nolint:ireturn // matches handleKey's tea.Model return.
func (m Model) exitKey(name string) (tea.Model, tea.Cmd) {
	if m.phase == phaseComplete && (name == keyEnter || name == keyEscape) {
		return m.backToList(), nil
	}
	return m.quitOr(name)
}

// Returns to the list from the complete screen, discarding the finished run's
// results and selection so the next run starts clean.
func (m Model) backToList() Model {
	m.phase = phaseList
	m.results = nil
	m.pending = 0
	m.selected = make(map[int]bool)
	m.cursor = 0
	return m
}

// Quits on q and otherwise leaves the model unchanged; it backs the
// non-interactive screens (help, executing, complete) where only exit applies.
//
//nolint:ireturn // matches handleKey's tea.Model return.
func (m Model) quitOr(name string) (tea.Model, tea.Cmd) {
	if name == keyQuit {
		return m, tea.Quit
	}
	return m, nil
}

// Handles keys on the list screen, delegating selection edits so no
// single function carries the whole keymap.
func (m Model) updateList(name string) (Model, tea.Cmd) {
	switch name {
	case keyQuit:
		return m, tea.Quit
	case keyRun:
		return m.startExecution()
	default:
		return m.editList(name), nil
	}
}

// Applies the non-terminal list keys: navigation, selection, search
// entry and mode cycling.
func (m Model) editList(name string) Model {
	switch name {
	case keySearch:
		m.searching = true
		m.searchBuf = ""
	case keyDown:
		m = m.moveCursor(1)
	case keyUp:
		m = m.moveCursor(-1)
	case keySpace, keyEnter:
		// Guard against an empty list, where the cursor has no MR to toggle and
		// would otherwise mark a hidden index that reappears once the filter clears.
		if len(m.visible()) > 0 {
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	case keySelectAll:
		m = m.selectAll()
	case keyClear:
		m.selected = make(map[int]bool)
	case keyMode:
		m.mode = (m.mode + 1) % modeCount
	}
	return m
}

// Shifts the cursor by delta, staying within the visible list.
func (m Model) moveCursor(delta int) Model {
	next := m.cursor + delta
	if next >= 0 && next < len(m.visible()) {
		m.cursor = next
	}
	return m
}

// Checks every visible MR.
func (m Model) selectAll() Model {
	for index := range m.visible() {
		m.selected[index] = true
	}
	return m
}

// Handles keys while the search prompt is open: enter commits the
// query (clearing any prior selection, since the indices it referenced no
// longer apply), esc discards it, and any other text edits the query.
func (m Model) updateSearch(keyMsg tea.KeyPressMsg) Model {
	switch keyMsg.String() {
	case keyEnter:
		m.filter = m.searchBuf
		m.filtered = filterMRs(m.mrs, m.filter)
		m.searching = false
		m.selected = make(map[int]bool)
		m.cursor = 0
	case keyEscape:
		m.searching = false
		m.searchBuf = ""
	case keyBackspace:
		if m.searchBuf != "" {
			runes := []rune(m.searchBuf)
			m.searchBuf = string(runes[:len(runes)-1])
		}
	default:
		m.searchBuf += keyMsg.Text
	}
	return m
}

// Kicks off the current mode's action against every selected MR
// concurrently, or stays on the list when nothing is selected.
func (m Model) startExecution() (Model, tea.Cmd) {
	selected := m.selectedMRs()
	if len(selected) == 0 {
		return m, nil
	}
	m.phase = phaseExecuting
	m.pending = len(selected)
	m.results = nil

	// A shared buffered channel caps how many of the batched commands hit glab
	// at once; the rest block on send until a slot frees.
	semaphore := make(chan struct{}, maxConcurrentActions)
	cmds := make([]tea.Cmd, 0, len(selected))
	for _, mergeRequest := range selected {
		cmds = append(cmds, m.actionCmd(mergeRequest, semaphore))
	}
	return m, tea.Batch(cmds...)
}

// Returns a command that applies the current mode to mergeRequest and
// reports the outcome. In approve & merge mode a failed approval skips the merge
// so a broken MR is not merged.
func (m Model) actionCmd(mergeRequest types.MR, semaphore chan struct{}) tea.Cmd {
	// Capture only what the command needs so it does not pin the whole Model
	// (its MR slices, results and selection map) alive while it runs.
	client, currentMode, method := m.client, m.mode, m.method
	return func() tea.Msg {
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		ctx := context.Background()
		var err error
		if currentMode == modeApprove || currentMode == modeApproveMerge {
			err = client.ApproveMR(ctx, mergeRequest.Project, mergeRequest.IID)
		}
		if err == nil && (currentMode == modeMerge || currentMode == modeApproveMerge) {
			err = client.MergeMR(ctx, mergeRequest.Project, mergeRequest.IID, method, true)
		}
		return mrResultMsg{mr: mergeRequest, err: err}
	}
}

// Returns the MRs that pass the current filter, in display order.
func (m Model) visible() []types.MR {
	return m.filtered
}

// Narrows mrs to those matching query; an empty query keeps them all.
func filterMRs(mrs []types.MR, query string) []types.MR {
	if query == "" {
		return mrs
	}
	lowered := strings.ToLower(query)
	var out []types.MR
	for _, mergeRequest := range mrs {
		if matchesFilter(mergeRequest, lowered) {
			out = append(out, mergeRequest)
		}
	}
	return out
}

// Returns the checked MRs in display order.
func (m Model) selectedMRs() []types.MR {
	var out []types.MR
	for index, mergeRequest := range m.visible() {
		if m.selected[index] {
			out = append(out, mergeRequest)
		}
	}
	return out
}

const helpText = `Keybindings
  j/k        move cursor
  space/enter toggle selection
  a/d        select all / clear
  /          search (enter to apply, esc to cancel)
  m          change mode (approve / merge / approve & merge)
  x          run the current mode on selected MRs
  ?          toggle this help
  q/ctrl+c   quit
`

func (m Model) renderResults() string {
	var builder strings.Builder
	builder.WriteString("Done:\n")
	for _, result := range m.results {
		mark := "✓"
		detail := ""
		if result.err != nil {
			mark = "✗"
			detail = ": " + result.err.Error()
		}
		fmt.Fprintf(&builder, "%s %s !%d - %s%s\n",
			mark, pathShorten(result.mr.Project), result.mr.IID, result.mr.Title, detail)
	}
	builder.WriteString("\n(enter: back to list, q: quit)\n")
	return builder.String()
}

func (m Model) renderList() string {
	var builder strings.Builder
	for index, mergeRequest := range m.visible() {
		builder.WriteString(m.renderRow(index, mergeRequest))
		builder.WriteByte('\n')
	}
	if m.searching {
		fmt.Fprintf(&builder, "\nsearch: %s\n", m.searchBuf)
	} else {
		fmt.Fprintf(&builder, "\nmode: %s  (m: change, x: run, ?: help, q: quit)\n", m.modeLabel())
	}
	return builder.String()
}

func (m Model) renderRow(index int, mergeRequest types.MR) string {
	cursor := " "
	if index == m.cursor {
		cursor = ">"
	}
	checkbox := "[ ]"
	if m.selected[index] {
		checkbox = "[x]"
	}
	warn := " "
	if mergeRequest.UnmergeableReason != "" {
		warn = "⚠"
	}
	return fmt.Sprintf("%s %s %s %s %s !%d - %s",
		cursor, checkbox, ciSymbol(mergeRequest.CIStatus), warn,
		pathShorten(mergeRequest.Project), mergeRequest.IID, mergeRequest.Title)
}

// Reports whether mergeRequest matches the already-lowercased
// query as a substring of its title, project path, or IID.
func matchesFilter(mergeRequest types.MR, lowered string) bool {
	return strings.Contains(strings.ToLower(mergeRequest.Title), lowered) ||
		strings.Contains(strings.ToLower(mergeRequest.Project), lowered) ||
		strings.Contains(strconv.Itoa(mergeRequest.IID), lowered)
}

// Maps a normalized pipeline status to a single-column glyph.
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
