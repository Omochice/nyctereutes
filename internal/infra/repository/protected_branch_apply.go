package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// Signals a re-protect that failed after its unprotect succeeded, which is the
// one outcome of an update that leaves the branch worse off than it started.
var errBranchLeftUnprotected = errors.New("branch was left unprotected")

// Creates, updates or removes one protected branch.
//
// An update unprotects and protects again rather than patching. GitLab's PATCH
// accepts push_access_level and merge_access_level, answers 200, and changes
// neither, confirmed against CE 19.2.1; a write path built on it would report
// success for a change it did not make. The cost is that the branch is
// unprotected between the two calls, which is why a failed re-protect says so.
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
		if err := a.unprotect(ctx, change.Name, field, branch.Name); err != nil {
			return err
		}
		if err := a.protect(ctx, change.Name, field, branch.Desired); err != nil {
			return fmt.Errorf("%w: %w", errBranchLeftUnprotected, err)
		}
		return nil
	case ChangeDelete:
		return a.unprotect(ctx, change.Name, field, branch.Name)
	default:
		return fmt.Errorf("%w: %s is not a protected branch change", errUnexpectedValueType, change.Type)
	}
}

// Protects a branch as the manifest declares it. Every attribute goes on the
// request: this is always a create, so an omitted one would take GitLab's
// default rather than the declared value.
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

// Removes a branch's protection. The name is percent-encoded because a
// protected branch is routinely named with a slash ("release/1.x",
// "release/*"), which an unencoded path splits into further segments; GitLab
// answers such a request 404, confirmed against CE 19.2.1.
func (a *Applier) unprotect(ctx context.Context, project, field, branch string) error {
	_, err := a.writer.Run(ctx, glabAPI,
		"projects/"+glab.EncodePath(project)+"/protected_branches/"+glab.EncodePath(branch),
		glabMethod, "DELETE")
	return wrapWrite(err, project, field)
}

// Names the branch as the field a failure is reported under, in the form the
// plan line uses.
func protectedBranchField(name string) string {
	return fmt.Sprintf("%s %q", protectedBranchPrefix, name)
}

// The form fields a protect sends. The roles go as the numbers GitLab wants,
// read from the one table the export reads, so a document and the write it
// produces cannot disagree about what a role means.
func protectedBranchForm(branch manifest.ProtectedBranch) ([]string, error) {
	push, err := branchAccessLevelNumber(branch.PushAccessLevel)
	if err != nil {
		return nil, err
	}
	merge, err := branchAccessLevelNumber(branch.MergeAccessLevel)
	if err != nil {
		return nil, err
	}
	return []string{
		"-f", fieldName + "=" + branch.Name,
		"-f", fieldPushAccessLevel + "=" + strconv.Itoa(push),
		"-f", fieldMergeAccessLevel + "=" + strconv.Itoa(merge),
		"-f", fieldAllowForcePush + "=" + strconv.FormatBool(branch.AllowForcePush),
	}, nil
}

// The number GitLab wants for a role a manifest names.
func branchAccessLevelNumber(name manifest.BranchAccessLevel) (int, error) {
	for _, role := range branchRoles() {
		if role.name == name {
			return role.number, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", errUnknownBranchAccessLevel, name)
}
