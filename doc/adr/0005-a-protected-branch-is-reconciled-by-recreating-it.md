# 5. A Protected Branch Is Reconciled by Recreating It

Date: 2026-11-11

## Status

Accepted

## Context

`spec.protected_branches` brings a project's push and merge rules into the manifest, and reconciling them means turning a difference between a declared protection and a live one into API calls.
Creating and removing a protection are answered by `POST /projects/:id/protected_branches` and `DELETE /projects/:id/protected_branches/:name`, and neither raises a question.
Changing one does.

GitLab's documentation states that `PATCH /projects/:id/protected_branches/:name` accepts `allow_force_push`, `code_owner_approval_required` and the `allowed_to_*` arrays, and that the scalar `push_access_level`, `merge_access_level` and `unprotect_access_level` parameters belong to the create alone.
What it does not state is what happens to a PATCH that sends one anyway.

Against GitLab CE 19.2.1 the answer is that nothing happens and nothing says so.
A `PATCH` carrying `push_access_level=40` against a branch holding 30 was answered `200`, and the body of that same response still reported 30.
The parameter was neither honoured nor refused.

The `allowed_to_*` arrays are the documented way to change an access level through PATCH.
They address an existing grant by the record id GitLab assigned it, take `{"id": n, "_destroy": true}` to remove one, and take an object without an id to add one.
A manifest holds no such id, so a write built on them would have to read the protection, pair each declared role with the grant record it corresponds to, and compose an array of edits and destructions.

One more thing was learned from the same instance.
CE reports no `unprotect_access_levels` at all: it accepts `unprotect_access_level` on create with a `201` and answers every subsequent read without it, "Allowed to unprotect" being a Premium feature.
That value can therefore be written and never read, so it is not in the manifest and is not something reconciliation has to preserve.

The same instance answers an unencoded branch name holding a slash, such as `release/1.x`, with `404` on both `GET` and `DELETE`.
A protected branch is routinely named with a slash, so an update that did not percent-encode the name would delete nothing and then have its create refused with `409`, because GitLab still holds the name.

The decision has to satisfy four forces.
A reported success has to be a change that happened, because an apply that says it changed a role and did not is worse than one that fails, since nothing afterwards contradicts it.
Each change type has one write path, because a second path taken for some differences and not others is a second thing to be wrong, and the plan cannot show which one a change will take.
The manifest holds no server-assigned value, and a write that needs one has to read it first, which makes the write depend on a read the plan already made and may no longer match.
A branch left unprotected has to be visible: whatever the design costs, the operator has to be able to see when it has been paid.

## Alternatives Considered

### Option 1: PATCH the scalars

An access-level change is sent as the scalar access-level parameters on a `PATCH` of the protection.

#### Pros

- It is the smallest change to write, and the one the shape of the API suggests.

#### Cons

- It is refused by the observation above: CE answers `200` and changes nothing, so every access-level update would be reported as applied and none of them would be.
- The failure is silent on both sides, since the plan is correct and the apply claims success.

### Option 2: PATCH the `allowed_to_*` arrays

An access-level change is sent as edits and `_destroy` entries in the `allowed_to_*` arrays, addressed by the grant record ids GitLab assigned.

#### Pros

- It works, and it never leaves the branch unprotected.

#### Cons

- A write acquires state: the applier has to read the protection to learn its grant record ids, pair each with the role the manifest declares, and send a mixture of edits and `_destroy` entries.
- That read is a second read of something the plan already read, and the ids can have changed in between.
- The applier would hold the only code in this repository that reasons about server-assigned identifiers, which [ADR-003](./0003-a-schedule-is-identified-by-its-description.md) took pains to keep out of the manifest, and it would do so for the sole benefit of a window measured in one round trip.

### Option 3: Unprotect and protect again

An update becomes the two calls that create and remove already use.

#### Pros

- There is no third code path and no state to carry between the two calls.
- GitLab addresses the protection by the name the manifest holds, so nothing has to be read first.

#### Cons

- Between the `DELETE` and the `POST` the branch is not protected, and if the `POST` fails it stays that way.
  The cost is real and cannot be argued away.
  A `DELETE` that fails costs nothing, because the branch keeps the protection it had.

## Decision

We will apply an access-level change by unprotecting the branch and protecting it again.

### 1. Every difference goes down the same path

A drift in `allow_force_push` alone could be a single PATCH, which GitLab does honour.
It is not made one.

#### Rationale

Two write paths would mean the plan line for an update no longer says what apply will do, and the cheaper path would be exercised only by the subset of changes that happen to touch nothing else.

### 2. The plan reports one change, not two

An update appears on the plan as one change, naming the attributes that differ.
The expansion into a `DELETE` and a `POST` happens inside apply.

#### Rationale

What the operator approves is the state they asked for, not the mechanics of reaching it.

### 3. A re-protect that fails says the branch was left open

When the second call fails, the report says that the branch was left unprotected, so it distinguishes "the change did not happen" from "the change did not happen and the branch is now unprotected".

#### Rationale

Nothing else in an apply report would mention that the branch was left unprotected, and the two call for different urgency.

### 4. Removals are planned before creations

Every removal is planned ahead of every creation, matching the order the schedules set.

#### Rationale

Here the order is load-bearing rather than conventional: a name GitLab still holds is answered `409`, so a run that created first could be refused for a protection it was about to remove.

### Not Decided Here

Nothing.

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
- The window is bounded by one request.
  A protection recreated with the same values is the same protection as far as the manifest is concerned, so a re-run of a failed apply restores it.

## References

- [ADR-003](./0003-a-schedule-is-identified-by-its-description.md), which settled how a child resource is identified when the manifest holds no id.
