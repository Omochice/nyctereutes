package repository

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// One protected branch as GitLab reports it.
// Unlike [LiveSchedule], no id is kept.
// GitLab addresses a protection by the name the manifest already holds, so a write has no use for the id.
//
// A CE build reports no unprotect_access_levels at all, an Enterprise build
// always does, and the difference is what the pointer on
// [manifest.ProtectedBranch] carries.
type LiveProtectedBranch struct {
	Name              string             `json:"name"`
	PushAccessLevels  []LiveBranchAccess `json:"push_access_levels"`
	MergeAccessLevels []LiveBranchAccess `json:"merge_access_levels"`
	AllowForcePush    bool               `json:"allow_force_push"`
	// Nil on a CE build, which leaves the field out of the response entirely, and
	// non-nil on an Enterprise one even when it holds no grant. An empty list is
	// what a project's automatic protection of its default branch reports.
	UnprotectAccessLevels []LiveBranchAccess `json:"unprotect_access_levels"`
}

// One grant inside a protected branch's access-level list.
// Only AccessLevel reaches the manifest; the other fields are read so a grant it cannot express is refused.
type LiveBranchAccess struct {
	AccessLevel int `json:"access_level"`
	// GitLab reports these as null when the grant is a plain role, so they are pointers.
	UserID       *int  `json:"user_id"`
	GroupID      *int  `json:"group_id"`
	DeployKeyID  *int  `json:"deploy_key_id"`
	MemberRoleID *int  `json:"member_role_id"`
	Inherited    *bool `json:"inherited"`
}

// Reads every protected branch a project owns, already described the way a manifest would.
// The endpoint pages, confirmed against GitLab CE 19.2.1.
// Described here rather than at each caller, so a read fails in the one place a read is made.
func (c *Client) FetchProtectedBranches(
	ctx context.Context, owner, name string,
) ([]manifest.ProtectedBranch, error) {
	out, err := c.runner.Run(ctx, "api", "--paginate",
		"projects/"+glab.EncodePath(owner+"/"+name)+"/protected_branches")
	if err != nil {
		return nil, fmt.Errorf("fetch protected branches %s/%s: %w", owner, name, err)
	}
	live, err := decodePages[LiveProtectedBranch](out, errNoProtectedBranchPage)
	if err != nil {
		return nil, fmt.Errorf("parse protected branches %s/%s: %w", owner, name, err)
	}
	if err := rejectIndescribableBranches(live); err != nil {
		return nil, fmt.Errorf("read protected branches %s/%s: %w", owner, name, err)
	}
	branches, err := toManifestProtectedBranches(live)
	if err != nil {
		return nil, fmt.Errorf("describe protected branches %s/%s: %w", owner, name, err)
	}
	return branches, nil
}

var errNoProtectedBranchPage = errors.New("response carries no protected branch list")

var errIndescribableBranch = errors.New("indescribable protected branch")

const (
	fieldPushAccessLevels      = "push_access_levels"
	fieldMergeAccessLevels     = "merge_access_levels"
	fieldUnprotectAccessLevels = "unprotect_access_levels"
)

// Refused rather than flattened to the first role.
// A document written from a flattened read would revoke the dropped grant once applied.
// An inherited protection is the group's to change, not the project's.
func rejectIndescribableBranches(branches []LiveProtectedBranch) error {
	for _, branch := range branches {
		for _, list := range []struct {
			field  string
			grants []LiveBranchAccess
		}{
			{field: fieldPushAccessLevels, grants: branch.PushAccessLevels},
			{field: fieldMergeAccessLevels, grants: branch.MergeAccessLevels},
		} {
			if err := rejectIndescribableGrants(branch.Name, list.field, list.grants); err != nil {
				return err
			}
		}
		// The unprotect list is the one that may legitimately hold nothing: a CE
		// build reports none, and an Enterprise build reports an empty one for the
		// protection a project creates with itself. More than one, or one naming
		// something no manifest can hold, is refused like the others.
		if len(branch.UnprotectAccessLevels) > 0 {
			if err := rejectIndescribableGrants(
				branch.Name, fieldUnprotectAccessLevels, branch.UnprotectAccessLevels,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectIndescribableGrants(branch, field string, grants []LiveBranchAccess) error {
	if len(grants) != 1 {
		return fmt.Errorf("%w: branch %q reports %d %s, want exactly one",
			errIndescribableBranch, branch, len(grants), field)
	}
	grant := grants[0]
	for _, named := range []struct {
		field string
		id    *int
	}{
		{field: "user_id", id: grant.UserID},
		{field: "group_id", id: grant.GroupID},
		{field: "deploy_key_id", id: grant.DeployKeyID},
		{field: "member_role_id", id: grant.MemberRoleID},
	} {
		if named.id != nil {
			return fmt.Errorf("%w: branch %q grants %s to %s %d, which no manifest can hold",
				errIndescribableBranch, branch, field, named.field, *named.id)
		}
	}
	if grant.Inherited != nil && *grant.Inherited {
		return fmt.Errorf("%w: branch %q is inherited from a group and is not the project's to declare",
			errIndescribableBranch, branch)
	}
	return nil
}

var errUnknownBranchAccessLevel = errors.New("unknown branch access level")

const (
	roleNoOne      manifest.BranchAccessLevel = "no_one"
	roleDeveloper  manifest.BranchAccessLevel = "developer"
	roleMaintainer manifest.BranchAccessLevel = "maintainer"
	roleAdmin      manifest.BranchAccessLevel = "admin"
)

// The numbers GitLab uses for the roles, confirmed against CE 19.2.1.
// It describes them as "No one", "Developers + Maintainers", "Maintainers" and "Instance admins".
const (
	accessNoOne      = 0
	accessDeveloper  = 30
	accessMaintainer = 40
	accessAdmin      = 60
)

type branchRole struct {
	name   manifest.BranchAccessLevel
	number int
}

// One table for both directions, so the export and the write cannot disagree about what a role means.
func branchRoles() []branchRole {
	return []branchRole{
		{name: roleNoOne, number: accessNoOne},
		{name: roleDeveloper, number: accessDeveloper},
		{name: roleMaintainer, number: accessMaintainer},
		{name: roleAdmin, number: accessAdmin},
	}
}

func branchAccessLevelName(level int) (manifest.BranchAccessLevel, error) {
	for _, role := range branchRoles() {
		if role.number == level {
			return role.name, nil
		}
	}
	return "", fmt.Errorf("%w: %d", errUnknownBranchAccessLevel, level)
}

// Sorted by name rather than left in GitLab's creation order, so the same live state produces the same file.
func toManifestProtectedBranches(live []LiveProtectedBranch) ([]manifest.ProtectedBranch, error) {
	if live == nil {
		return nil, nil
	}
	branches := make([]manifest.ProtectedBranch, 0, len(live))
	for _, branch := range live {
		converted, err := toManifestProtectedBranch(branch)
		if err != nil {
			return nil, err
		}
		branches = append(branches, converted)
	}
	slices.SortFunc(branches, func(left, right manifest.ProtectedBranch) int {
		return cmp.Compare(left.Name, right.Name)
	})
	return branches, nil
}

// Each grant list holds exactly one entry, because [rejectIndescribableBranches] ran first.
func toManifestProtectedBranch(live LiveProtectedBranch) (manifest.ProtectedBranch, error) {
	push, err := branchAccessLevelName(live.PushAccessLevels[0].AccessLevel)
	if err != nil {
		return manifest.ProtectedBranch{}, fmt.Errorf("branch %q %s: %w", live.Name, fieldPushAccessLevels, err)
	}
	merge, err := branchAccessLevelName(live.MergeAccessLevels[0].AccessLevel)
	if err != nil {
		return manifest.ProtectedBranch{}, fmt.Errorf("branch %q %s: %w", live.Name, fieldMergeAccessLevels, err)
	}
	described := manifest.ProtectedBranch{
		Name:             live.Name,
		PushAccessLevel:  push,
		MergeAccessLevel: merge,
		AllowForcePush:   live.AllowForcePush,
	}
	// A CE build omits the field and an Enterprise one reports an empty list for
	// a protection created with its project. Neither states a role, and a
	// manifest naming one where GitLab states none would report drift no apply
	// could settle, so both leave the key unset.
	if len(live.UnprotectAccessLevels) > 0 {
		role, err := branchAccessLevelName(live.UnprotectAccessLevels[0].AccessLevel)
		if err != nil {
			return manifest.ProtectedBranch{},
				fmt.Errorf("branch %q %s: %w", live.Name, fieldUnprotectAccessLevels, err)
		}
		described.UnprotectAccessLevel = new(manifest.BranchUnprotectAccessLevel(role))
	}
	return described, nil
}
