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

// One protected branch as GitLab reports it. Unlike [LiveSchedule], no id is
// kept: GitLab addresses a protection by the name the manifest already holds,
// so the server-assigned id is of no use to a write.
//
// GitLab CE reports no unprotect_access_levels and no code_owner_approval_
// required, both being Premium features, so neither is read.
type LiveProtectedBranch struct {
	Name              string             `json:"name"`
	PushAccessLevels  []LiveBranchAccess `json:"push_access_levels"`
	MergeAccessLevels []LiveBranchAccess `json:"merge_access_levels"`
	AllowForcePush    bool               `json:"allow_force_push"`
}

// One grant inside a protected branch's access-level list. Only AccessLevel is
// describable by a manifest; the rest are read solely so a grant the manifest
// cannot express is refused instead of being flattened to the role beside it.
type LiveBranchAccess struct {
	AccessLevel int `json:"access_level"`
	// GitLab reports these as null when the grant is a plain role, so they are
	// pointers: a zero id and an absent one are not the same answer.
	UserID       *int  `json:"user_id"`
	GroupID      *int  `json:"group_id"`
	DeployKeyID  *int  `json:"deploy_key_id"`
	MemberRoleID *int  `json:"member_role_id"`
	Inherited    *bool `json:"inherited"`
}

// Reads every protected branch a project owns. The endpoint pages the way the
// schedules one does, confirmed against GitLab CE 19.2.1, so glab is asked to
// follow the pages rather than the project being described by its first page.
func (c *Client) FetchProtectedBranches(ctx context.Context, owner, name string) ([]LiveProtectedBranch, error) {
	out, err := c.runner.Run(ctx, "api", "--paginate",
		"projects/"+glab.EncodePath(owner+"/"+name)+"/protected_branches")
	if err != nil {
		return nil, fmt.Errorf("fetch protected branches %s/%s: %w", owner, name, err)
	}
	branches, err := decodePages[LiveProtectedBranch](out, errNoProtectedBranchPage)
	if err != nil {
		return nil, fmt.Errorf("parse protected branches %s/%s: %w", owner, name, err)
	}
	if err := rejectIndescribableBranches(branches); err != nil {
		return nil, fmt.Errorf("read protected branches %s/%s: %w", owner, name, err)
	}
	return branches, nil
}

// Signals output that carries no page of protected branches to read.
var errNoProtectedBranchPage = errors.New("response carries no protected branch list")

// Signals a protection whose grants a manifest cannot express.
var errIndescribableBranch = errors.New("indescribable protected branch")

// The two access-level lists, named so a refusal says which one it read.
const (
	fieldPushAccessLevels  = "push_access_levels"
	fieldMergeAccessLevels = "merge_access_levels"
)

// Rejects a protection the manifest could not describe, rather than reading it
// as the role it happens to list first. A branch granted to a user, a group, a
// deploy key or a custom role holds more than one grant, or a grant naming
// something no manifest carries, and applying a document written from a
// flattened read would revoke the grant that was dropped.
//
// Deploy keys work in the free tier, so this is a shape a CE project really
// reaches; the others need Premium. An inherited protection comes from a group
// and is not the project's to change.
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
	}
	return nil
}

// Reports why one access-level list cannot be described, or nil when it holds
// exactly one plain role.
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

// Signals an access level GitLab reports that no manifest role names.
var errUnknownBranchAccessLevel = errors.New("unknown branch access level")

// The roles a manifest names, spelled here so the table below and any write can
// refer to one rather than repeat the string.
const (
	roleNoOne      manifest.BranchAccessLevel = "no_one"
	roleDeveloper  manifest.BranchAccessLevel = "developer"
	roleMaintainer manifest.BranchAccessLevel = "maintainer"
	roleAdmin      manifest.BranchAccessLevel = "admin"
)

// The numbers GitLab uses for those roles, confirmed against CE 19.2.1, which
// describes them as "No one", "Developers + Maintainers", "Maintainers" and
// "Instance admins".
const (
	accessNoOne      = 0
	accessDeveloper  = 30
	accessMaintainer = 40
	accessAdmin      = 60
)

// One role a manifest names, paired with the number GitLab uses for it.
type branchRole struct {
	name   manifest.BranchAccessLevel
	number int
}

// The whole table, read by both directions so the export and the write cannot
// disagree about what a role means.
func branchRoles() []branchRole {
	return []branchRole{
		{name: roleNoOne, number: accessNoOne},
		{name: roleDeveloper, number: accessDeveloper},
		{name: roleMaintainer, number: accessMaintainer},
		{name: roleAdmin, number: accessAdmin},
	}
}

// Names the role GitLab's number stands for. An unrecognized number is refused
// rather than defaulted, so a role a later GitLab adds is reported instead of
// silently read as one of these.
func branchAccessLevelName(level int) (manifest.BranchAccessLevel, error) {
	for _, role := range branchRoles() {
		if role.number == level {
			return role.name, nil
		}
	}
	return "", fmt.Errorf("%w: %d", errUnknownBranchAccessLevel, level)
}

// Converts the live protections into manifest documents, ordered by name.
// GitLab lists them in creation order, so a protection removed and recreated
// would move; a manifest is a file under version control, so the same live
// state has to produce the same bytes. The name is unique, which makes the
// order total.
//
// Protections that were never read pass through as nil, which the field on
// [manifest.RepositorySpec] keeps distinct from the empty list.
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

// Describes one live protection the way a manifest would, which is what makes
// it comparable with a declared one. The grants are known to hold exactly one
// entry each, because [rejectIndescribableBranches] ran first.
func toManifestProtectedBranch(live LiveProtectedBranch) (manifest.ProtectedBranch, error) {
	push, err := branchAccessLevelName(live.PushAccessLevels[0].AccessLevel)
	if err != nil {
		return manifest.ProtectedBranch{}, fmt.Errorf("branch %q %s: %w", live.Name, fieldPushAccessLevels, err)
	}
	merge, err := branchAccessLevelName(live.MergeAccessLevels[0].AccessLevel)
	if err != nil {
		return manifest.ProtectedBranch{}, fmt.Errorf("branch %q %s: %w", live.Name, fieldMergeAccessLevels, err)
	}
	return manifest.ProtectedBranch{
		Name:             live.Name,
		PushAccessLevel:  push,
		MergeAccessLevel: merge,
		AllowForcePush:   live.AllowForcePush,
	}, nil
}
