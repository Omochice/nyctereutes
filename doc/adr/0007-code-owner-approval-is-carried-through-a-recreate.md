# 7. Code Owner Approval Is Carried Through a Recreate

Date: 2026-11-12

## Status

Accepted

## Context

[ADR-005](./0005-a-protected-branch-is-reconciled-by-recreating-it.md) applies every change to a protected branch by unprotecting the branch and protecting it again.
[ADR-006](./0006-the-unprotect-access-level-is-reconciled-only-where-gitlab-reports-it.md) then decided that `code_owner_approval_required` is not a manifest key, because an Enterprise build without a licence reports the field and refuses the value.
Its rationale adds that nothing a recreate does can lose a value the instance will not hold.
That is true of the instance it describes, and it says nothing about an instance that does hold the value.

A licensed Enterprise instance holds it.
The create sent only what a manifest can state, so it never carried the setting.
The GitLab 19.5.3-ee sources say what such a create does: the setting is taken only when the request carries it and the feature is licensed, and the column behind it defaults to `false`.
A role change applied to a branch that requires code owner approval would therefore bring the branch back without it, while the plan showed the role alone.
That is the unannounced change of a setting the manifest never mentioned, which ADR-006, Decision 2 (A recreate sends the live level when the manifest is silent) already ruled out for the unprotect level.

The reset on a licensed instance was not observed.
The same sources show an instance without a licence reporting `false` whatever is stored, and the instances at hand were GitLab CE 19.5.3 and GitLab EE 19.5.3 without a licence, so the state that loses the setting could not be reached.
What was observed, on GitLab EE 19.5.3 without a licence, is that a create carrying `code_owner_approval_required=true` is answered `201` and reports `false`.

The decision has to keep a recreate from turning the setting off, has to leave ADR-006, Decision 6 (Code owner approval stays out of the manifest) standing, and may not change what is sent to an instance that never reports the setting as on, because nothing about such an instance called for a change.

## Alternatives Considered

### Option 1: Leave the recreate as it is

The create sends what the manifest states and nothing else.

#### Pros

- The applier sends only values the plan showed.

#### Cons

- On a licensed instance every role change turns code owner approval off, and neither the plan nor the report says so.

### Option 2: Make the setting a manifest key

The setting is declared, compared and sent like the roles.

#### Pros

- The value a recreate sends is one the document states and the plan shows.

#### Cons

- ADR-006, Decision 6 (Code owner approval stays out of the manifest) rejected it: on an Enterprise build without a licence a declaration could be neither applied nor confirmed.

### Option 3: Send the setting on every create, on or off

Every create carries the setting the instance reported, or `false` where there is nothing to carry.

#### Pros

- One shape of request for every create.

#### Cons

- Every create on every instance gains a parameter, although only an instance that reported the setting as on has a value to keep.

### Option 4: Send the setting on a recreate when the instance reported it as on

The setting is read with the protections.
A recreate of a branch the instance reported it on for sends it again, and no other request changes.

#### Pros

- A recreate on a licensed instance keeps the setting.
- A request to an instance that never reports the setting as on stays what it was.
- The manifest is untouched.

#### Cons

- The applier sends a value that appears in no document and on no plan.

## Decision

We will take Option 4.

### 1. The setting is read although no manifest key names it

Code owner approval is read from the instance together with the protections, and is kept apart from what an export writes.

#### Rationale

A recreate has to learn the value from somewhere, and ADR-006, Decision 6 (Code owner approval stays out of the manifest) leaves the instance as the only place that holds it.
Keeping it apart from the exported protections is what stops the read from turning the setting into a key by another route.

### 2. A recreate sends the setting again when the instance reported it as on

When a protection is recreated and the instance reported that the branch requires code owner approval, the create asks for it again.

#### Rationale

Without it the create takes GitLab's default, and the recreate becomes the unannounced change ADR-005 exists to prevent.
This is the reasoning of ADR-006, Decision 2 (A recreate sends the live level when the manifest is silent), applied to a setting that has no key to be silent about.

### 3. Nothing is sent where the instance did not report the setting as on

A recreate of a branch the instance reported the setting off for, and every plain create, sends what it sent before this decision.

#### Rationale

Off is what a create gives when the setting is not mentioned, so there is no value to keep by sending it.
A Community Edition build and an Enterprise build without a licence never report the setting as on, so the decision reaches only the instances on which the loss can happen.

### Not Decided Here

- Other settings GitLab reports that no manifest key names: this ADR covers code owner approval alone, and another will be decided when one is found that a recreate loses.

## Consequences

### Positive

- A role change on a licensed instance keeps code owner approval, as far as the GitLab sources say what a create does.
- Nothing changes for a Community Edition instance or an Enterprise one without a licence.
- The manifest and its schema stay as ADR-006 left them.

### Negative

- A re-run after a failed re-protect does not bring the setting back: the branch is then protected by a plain create, and the instance no longer holds a value to carry.
  ADR-005's mitigation, that a re-run of a failed apply restores the protection, holds only for what the manifest states.
- The applier sends a value that the manifest, the plan and the export never show.
- The decision rests on reading GitLab's sources: neither the loss nor its prevention was observed on a licensed instance.

### Mitigations

- A failed re-protect is reported as a branch left unprotected, which sends the operator to look at the branch; the report does not mention code owner approval.
- The value sent is the one the instance reported in the same run, so the recreate leaves the setting as it found it, and the user documentation says that the setting is read and sent again.
- If a licensed instance refused the parameter, the re-protect would fail and be reported as a branch left unprotected, not pass in silence.

## References

- [ADR-005](./0005-a-protected-branch-is-reconciled-by-recreating-it.md), whose recreate this decision keeps from turning the setting off.
- [ADR-006](./0006-the-unprotect-access-level-is-reconciled-only-where-gitlab-reports-it.md), whose Decision 6 this decision leaves standing and whose rationale for it covers only an instance without a licence.
- [Protected branches API](https://docs.gitlab.com/api/protected_branches/).
