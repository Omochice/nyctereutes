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

// Every attribute goes on the request, because a create takes GitLab's default for an omitted one.
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

func branchAccessLevelNumber(name manifest.BranchAccessLevel) (int, error) {
	for _, role := range branchRoles() {
		if role.name == name {
			return role.number, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", errUnknownBranchAccessLevel, name)
}
