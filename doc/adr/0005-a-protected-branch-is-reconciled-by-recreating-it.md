# 5. A Protected Branch Is Reconciled by Recreating It

Date: 2026-11-11

## Status

Accepted

## Context

`spec.protected_branches` brings a project's push and merge rules into the manifest, and reconciling them means turning a difference between a declared protection and a live one into API calls. Creating and removing a protection are answered by `POST /projects/:id/protected_branches` and `DELETE /projects/:id/protected_branches/:name`, and neither raises a question. Changing one does.

GitLab's documentation states that `PATCH /projects/:id/protected_branches/:name` accepts `allow_force_push`, `code_owner_approval_required` and the `allowed_to_*` arrays, and that the scalar `push_access_level`, `merge_access_level` and `unprotect_access_level` parameters belong to the create alone. What it does not state is what happens to a PATCH that sends one anyway.

Against GitLab CE 19.2.1 the answer is that nothing happens and nothing says so. A `PATCH` carrying `push_access_level=40` against a branch holding 30 was answered `200`, and the body of that same response still reported 30. The parameter was neither honoured nor refused. The capture is in `.momomo/ai/gitlab/capture/protected-branches/patch-scalar.json`.

The `allowed_to_*` arrays are the documented way to change an access level through PATCH. They address an existing grant by the record id GitLab assigned it, take `{"id": n, "_destroy": true}` to remove one, and take an object without an id to add one. A manifest holds no such id, so a write built on them would have to read the protection, pair each declared role with the grant record it corresponds to, and compose an array of edits and destructions.

One more thing was learned from the same instance. CE reports no `unprotect_access_levels` at all: it accepts `unprotect_access_level` on create with a `201` and answers every subsequent read without it, "Allowed to unprotect" being a Premium feature. That value can therefore be written and never read, so it is not in the manifest and is not something reconciliation has to preserve.

## Decision Drivers

- **A reported success is a change that happened**: an apply that says it changed a role and did not is worse than one that fails, because nothing afterwards contradicts it.
- **One write path per change type**: a second path taken for some differences and not others is a second thing to be wrong, and the plan cannot show which one a change will take.
- **The manifest holds no server-assigned value**: a write that needs one has to read it first, which makes the write depend on a read the plan already made and may no longer match.
- **A branch left unprotected is visible**: whatever the design costs, the operator has to be able to see when it has been paid.

## Considered Options

### Option 1: PATCH the scalars

The smallest change to write, and the one the shape of the API suggests. It is refused by the observation above: CE answers `200` and changes nothing, so every access-level update would be reported as applied and none of them would be. The failure is silent on both sides, since the plan is correct and the apply claims success.

### Option 2: PATCH the `allowed_to_*` arrays

This works, and it never leaves the branch unprotected. The cost is that a write acquires state: the applier has to read the protection to learn its grant record ids, pair each with the role the manifest declares, and send a mixture of edits and `_destroy` entries. That read is a second read of something the plan already read, and the ids can have changed in between. The applier would hold the only code in this repository that reasons about server-assigned identifiers, which [ADR-003](./0003-a-schedule-is-identified-by-its-description.md) took pains to keep out of the manifest, and it would do so for the sole benefit of a window measured in one round trip.

### Option 3: Unprotect and protect again

An update becomes the two calls that create and remove already use, so there is no third code path and no state to carry between them. GitLab addresses the protection by the name the manifest holds, so nothing has to be read first.

The cost is real and cannot be argued away: between the `DELETE` and the `POST` the branch is not protected, and if the `POST` fails it stays that way. A `DELETE` that fails costs nothing, because the branch keeps the protection it had.

## Decision

An access-level change is applied by unprotecting the branch and protecting it again.

### 1. Every difference goes down the same path

A drift in `allow_force_push` alone could be a single PATCH, which GitLab does honour. It is not made one. Two write paths would mean the plan line for an update no longer says what apply will do, and the cheaper path would be exercised only by the subset of changes that happen to touch nothing else.

### 2. The plan reports one change, not two

An update is one `ChangeUpdate` on the plan, naming the attributes that differ. The expansion into a `DELETE` and a `POST` happens inside apply. What the operator approves is the state they asked for, not the mechanics of reaching it.

### 3. A re-protect that fails says the branch was left open

`errBranchLeftUnprotected` wraps the failure of the second call, so the report distinguishes "the change did not happen" from "the change did not happen and the branch is now unprotected". Nothing else in an apply report would mention it, and the two call for different urgency.

### 4. Removals are planned before creations

`diffProtectedBranches` emits every removal ahead of every creation, matching the order the schedules set. Here it is load-bearing rather than conventional: a name GitLab still holds is answered `409`, so a run that created first could be refused for a protection it was about to remove.

## Consequences

### Positive

- An access-level change that is reported as applied was applied, because the create is the call that carries the value.
- The applier holds no server-assigned identifier and makes no read of its own, so an apply performs exactly the changes the plan showed.
- Creating, changing and removing a protection are three arrangements of two calls, so there is one thing to get right rather than three.

### Negative

- A branch is unprotected for the duration of one round trip on every access-level change, and stays unprotected if the re-protect fails.
- The protection's server-assigned id changes on every update, along with anything GitLab keeps against it.
- An update makes two calls where one would have done for `allow_force_push`.

### Mitigations

- The failed re-protect is named as its own condition rather than folded into a generic write failure, so the report tells the operator to go and look.
- The window is bounded by one request. A protection recreated with the same values is the same protection as far as the manifest is concerned, so a re-run of a failed apply restores it.

## Implementation Notes

- `applyProtectedBranch` in `internal/infra/repository/protected_branch_apply.go` holds the three arrangements, and `protect` and `unprotect` are the two calls.
- The branch name is percent-encoded on every path. A protected branch is routinely named with a slash, and CE 19.2.1 answers an unencoded `release/1.x` with `404` on both `GET` and `DELETE`, which would leave an update deleting nothing and then refused by the `409` above.
- The roles a manifest names and the numbers GitLab wants live in one table, `branchRoles` in `internal/infra/repository/protected_branch.go`, read by both the export and the write.

## References

- The captured responses and what they showed: `.momomo/ai/gitlab/capture/protected-branches/README.md`.
- [ADR-003](./0003-a-schedule-is-identified-by-its-description.md), which settled how a child resource is identified when the manifest holds no id.
