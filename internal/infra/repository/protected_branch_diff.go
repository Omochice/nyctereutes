package repository

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The field every protected-branch change is reported under, which is how [Applier] tells one from a project setting.
const fieldProtectedBranches = "protected_branches"

// How one protection is named on a plan line and in the error of a write that failed, shared so the two can be matched.
const protectedBranchPrefix = "protected_branch"

// The protected branch one [Change] acts on.
// Only the side the change type implies is filled: a creation has no Live, a deletion no Desired.
// Name is held separately because it is what every change carries.
// It is also what pairs a declared protection with a live one.
type ProtectedBranchChange struct {
	Name    string
	Desired manifest.ProtectedBranch
	Live    manifest.ProtectedBranch
}

// Reports how a project's live protections differ from the declared ones, pairing them by name.
//
// Deletions come first for the reason [diffSchedules] gives, and for a second one here.
// An update is applied by unprotecting and protecting again.
// A run that created before deleting would meet a name GitLab still holds.
func diffProtectedBranches(name string, desired, live []manifest.ProtectedBranch) []Change {
	declared := make(map[string]struct{}, len(desired))
	for _, want := range desired {
		declared[want.Name] = struct{}{}
	}

	changes := make([]Change, 0, len(desired)+len(live))
	byName := make(map[string]manifest.ProtectedBranch, len(live))
	for _, have := range live {
		byName[have.Name] = have
		if _, kept := declared[have.Name]; !kept {
			changes = append(changes, protectedBranchChange(ChangeDelete, name, ProtectedBranchChange{
				Name: have.Name,
				Live: have,
			}))
		}
	}

	for _, want := range desired {
		have, exists := byName[want.Name]
		switch {
		case !exists:
			changes = append(changes, protectedBranchChange(ChangeCreate, name, ProtectedBranchChange{
				Name:    want.Name,
				Desired: want,
			}))
		case have != want:
			changes = append(changes, protectedBranchChange(ChangeUpdate, name, ProtectedBranchChange{
				Name:    want.Name,
				Desired: want,
				Live:    have,
			}))
		}
	}
	return changes
}

// Wraps one protection as a [Change], so the three producers cannot disagree about the field it is reported under.
func protectedBranchChange(kind ChangeType, name string, branch ProtectedBranchChange) Change {
	return Change{Type: kind, Name: name, Field: fieldProtectedBranches, ProtectedBranch: &branch}
}

// The protected-branch attribute names, repeating the manifest keys because a struct tag cannot name a constant.
const (
	fieldPushAccessLevel  = "push_access_level"
	fieldMergeAccessLevel = "merge_access_level"
	fieldAllowForcePush   = "allow_force_push"
)

type protectedBranchAttribute struct {
	field string
	value string
}

// Renders a protection's attributes in a fixed order, so two of them line up position by position.
// The name is absent because it is the identity: a protection of another branch is a different one, not a changed one.
func protectedBranchAttributes(branch manifest.ProtectedBranch) []protectedBranchAttribute {
	return []protectedBranchAttribute{
		{field: fieldPushAccessLevel, value: string(branch.PushAccessLevel)},
		{field: fieldMergeAccessLevel, value: string(branch.MergeAccessLevel)},
		{field: fieldAllowForcePush, value: strconv.FormatBool(branch.AllowForcePush)},
	}
}

// Renders the change as the lines a plan shows for it.
// A creation lists every attribute because none of them exists yet.
// An update lists only those that differ, and a deletion none.
// What a removal costs is the protection itself, which the header already names.
func (b *ProtectedBranchChange) line(kind ChangeType) string {
	header := fmt.Sprintf("%s %q", protectedBranchPrefix, b.Name)
	switch kind {
	case ChangeCreate:
		declared := protectedBranchAttributes(b.Desired)
		lines := make([]string, 0, len(declared)+1)
		lines = append(lines, "+ "+header)
		for _, want := range declared {
			lines = append(lines, fmt.Sprintf("    + %s: %s", want.field, want.value))
		}
		return strings.Join(lines, "\n")
	case ChangeUpdate:
		live := protectedBranchAttributes(b.Live)
		lines := make([]string, 0, len(live)+1)
		lines = append(lines, "~ "+header)
		for index, want := range protectedBranchAttributes(b.Desired) {
			if live[index].value != want.value {
				lines = append(lines, fmt.Sprintf("    ~ %s: %s → %s", want.field, live[index].value, want.value))
			}
		}
		return strings.Join(lines, "\n")
	case ChangeDelete:
		return "- " + header
	default:
		return ""
	}
}
