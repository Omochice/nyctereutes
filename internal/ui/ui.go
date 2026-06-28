// Package ui renders dependency merge requests as text tables and action
// messages, writing to an injected [io.Writer] so output can be captured in
// tests.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Omochice/nyctereutes/internal/types"
)

// tabPadding is the cell padding passed to tabwriter.
const tabPadding = 2

// UI renders to w. multiProject controls whether action messages are prefixed
// with the project path (only meaningful when MRs span several projects).
type UI struct {
	w            io.Writer
	multiProject bool
	json         bool
}

// New builds a UI for a flat MR list, auto-detecting whether the MRs span
// multiple projects.
func New(w io.Writer, mrs []types.MR, jsonOut bool) *UI {
	return &UI{w: w, multiProject: isMultiProject(mrs), json: jsonOut}
}

// NewFromGroups builds a UI for grouped MRs, detecting multi-project output
// across all groups.
func NewFromGroups(w io.Writer, groups map[string][]types.MR, jsonOut bool) *UI {
	return &UI{w: w, multiProject: isMultiProjectGroups(groups), json: jsonOut}
}

func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, tabPadding, ' ', 0)
}

// DisplayList prints MRs as a flat table, or as a JSON array under json mode.
func (u *UI) DisplayList(mrs []types.MR) error {
	if u.json {
		// A nil slice marshals to null; emit [] so consumers can always iterate.
		if mrs == nil {
			mrs = []types.MR{}
		}
		return u.writeJSON(mrs)
	}
	if len(mrs) == 0 {
		return nil
	}

	tabWriter := newTabWriter(u.w)
	_, _ = fmt.Fprintln(tabWriter, "PROJECT\tMR\tTITLE")
	for _, mr := range mrs {
		_, _ = fmt.Fprintf(tabWriter, "%s\t!%d\t%s\n", mr.Project, mr.IID, mr.Title)
	}
	return flush(tabWriter)
}

// DisplayGroups sorts groups alphabetically and MRs within a group by project
// then IID, so output is stable across runs.
func (u *UI) DisplayGroups(groups map[string][]types.MR) error {
	if u.json {
		return u.writeJSON(groups)
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tabWriter := newTabWriter(u.w)
	_, _ = fmt.Fprintln(tabWriter, "GROUP\tPROJECT\tMR\tURL")
	for _, key := range keys {
		groupMRs := append([]types.MR(nil), groups[key]...)
		sort.Slice(groupMRs, func(i, j int) bool {
			if groupMRs[i].Project != groupMRs[j].Project {
				return groupMRs[i].Project < groupMRs[j].Project
			}
			return groupMRs[i].IID < groupMRs[j].IID
		})

		for i, groupMR := range groupMRs {
			groupCell := ""
			if i == 0 {
				groupCell = key
			}
			parts := strings.Split(groupMR.Project, "/")
			_, _ = fmt.Fprintf(tabWriter, "%s\t%s\t!%d\t%s\n", groupCell, parts[len(parts)-1], groupMR.IID, groupMR.URL)
		}
	}
	return flush(tabWriter)
}

// PrintAction prints a standardized action message for an MR, prefixed with the
// project when output spans multiple projects.
func (u *UI) PrintAction(action string, mr types.MR, details ...string) {
	message := fmt.Sprintf("%s !%d", action, mr.IID)
	if len(details) > 0 {
		message += ": " + strings.Join(details, "; ")
	}
	_, _ = fmt.Fprintf(u.w, "%s%s\n", u.prefix(mr), message)
}

// PrintError prints a per-MR failure line, prefixed with the project when
// output spans multiple projects.
func (u *UI) PrintError(action string, mr types.MR, err error) {
	_, _ = fmt.Fprintf(u.w, "%sfailed to %s !%d: %v\n", u.prefix(mr), action, mr.IID, err)
}

func (u *UI) prefix(mr types.MR) string {
	if u.multiProject {
		return fmt.Sprintf("[%s] ", mr.Project)
	}
	return ""
}

func (u *UI) writeJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	if _, err := fmt.Fprintln(u.w, string(data)); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}
	return nil
}

func flush(tabWriter *tabwriter.Writer) error {
	if err := tabWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush table: %w", err)
	}
	return nil
}

func isMultiProject(mrs []types.MR) bool {
	for i := range mrs {
		if mrs[i].Project != mrs[0].Project {
			return true
		}
	}
	return false
}

func isMultiProjectGroups(groups map[string][]types.MR) bool {
	for _, mrs := range groups {
		if isMultiProject(mrs) {
			return true
		}
	}
	return false
}
