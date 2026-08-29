package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The one outcome of an update that leaves the branch worse off than it started.
var errBranchLeftUnprotected = errors.New("branch was left unprotected")

// An update unprotects and protects again rather than patching.
// GitLab's PATCH accepts both access levels, answers 200, and changes neither, confirmed against CE 19.2.1.
// The branch is unprotected between the two calls, which is why a failed re-protect says so.
func (a *Applier) applyProtectedBranch(ctx context.Context, change Change) error {
	branch := change.ProtectedBranch
	if branch == nil {
		return fmt.Errorf("%w: %s carries no protected branch", errUnexpectedValueType, change.Field)
	}
	field := protectedBranchField(branch.Name)
	switch change.Type {
	case ChangeCreate:
		return a.protect(ctx, change.Name, field, branch.Desired)
	case ChangeUpdate:
		want := carryUnstatedUnprotect(branch.Desired, branch.Live)
		if err := a.unprotect(ctx, change.Name, field, branch.Name); err != nil {
			return err
		}
		if err := a.protect(ctx, change.Name, field, want); err != nil {
			return fmt.Errorf("%w: %w", errBranchLeftUnprotected, err)
		}
		return nil
	case ChangeDelete:
		return a.unprotect(ctx, change.Name, field, branch.Name)
	default:
		return fmt.Errorf("%w: %s is not a protected branch change", errUnexpectedValueType, change.Type)
	}
}

// Puts the live unprotect level onto a declaration that is silent about it, so
// recreating the protection does not quietly change a setting the manifest
// never spoke for. Without this an update would send no unprotect level and
// GitLab would grant its own default, which on an Enterprise build demotes a
// branch only instance admins could unprotect down to maintainers.
func carryUnstatedUnprotect(desired, live manifest.ProtectedBranch) manifest.ProtectedBranch {
	if desired.UnprotectAccessLevel == nil {
		desired.UnprotectAccessLevel = live.UnprotectAccessLevel
	}
	return desired
}

// Protects a branch as the manifest declares it. Every stated attribute goes on
// the request: this is always a create, so an omitted one would take GitLab's
// default rather than the declared value. An unprotect level nobody stated is
// left off, because a CE build refuses to report it and would have nothing to
// compare a sent value against.
func (a *Applier) protect(
	ctx context.Context, project, field string, branch manifest.ProtectedBranch,
) error {
	form, err := protectedBranchForm(branch)
	if err != nil {
		return err
	}
	args := append([]string{
		glabAPI, "projects/" + glab.EncodePath(project) + "/protected_branches",
		glabMethod, "POST",
	}, form...)
	_, err = a.writer.Run(ctx, args...)
	return wrapWrite(err, project, field)
}

// The name is percent-encoded: a slash left in it splits the path, which GitLab CE 19.2.1 answers with 404.
func (a *Applier) unprotect(ctx context.Context, project, field, branch string) error {
	_, err := a.writer.Run(ctx, glabAPI,
		"projects/"+glab.EncodePath(project)+"/protected_branches/"+glab.EncodePath(branch),
		glabMethod, "DELETE")
	return wrapWrite(err, project, field)
}

func protectedBranchField(name string) string {
	return fmt.Sprintf("%s %q", protectedBranchPrefix, name)
}

// The widest a protect's form gets: four attributes and the optional unprotect
// level, each as a flag and a value.
const protectedBranchFormSize = 10

func protectedBranchForm(branch manifest.ProtectedBranch) ([]string, error) {
	push, err := branchAccessLevelNumber(branch.PushAccessLevel)
	if err != nil {
		return nil, err
	}
	merge, err := branchAccessLevelNumber(branch.MergeAccessLevel)
	if err != nil {
		return nil, err
	}
	form := make([]string, 0, protectedBranchFormSize)
	form = append(form,
		"-f", fieldName+"="+branch.Name,
		"-f", fieldPushAccessLevel+"="+strconv.Itoa(push),
		"-f", fieldMergeAccessLevel+"="+strconv.Itoa(merge),
		"-f", fieldAllowForcePush+"="+strconv.FormatBool(branch.AllowForcePush),
	)
	if branch.UnprotectAccessLevel == nil {
		return form, nil
	}
	unprotect, err := branchAccessLevelNumber(manifest.BranchAccessLevel(*branch.UnprotectAccessLevel))
	if err != nil {
		return nil, err
	}
	return append(form, "-f", fieldUnprotectAccessLevel+"="+strconv.Itoa(unprotect)), nil
}

func branchAccessLevelNumber(name manifest.BranchAccessLevel) (int, error) {
	for _, role := range branchRoles() {
		if role.name == name {
			return role.number, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", errUnknownBranchAccessLevel, name)
}
