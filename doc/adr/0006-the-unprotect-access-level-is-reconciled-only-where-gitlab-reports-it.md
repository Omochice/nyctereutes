# 6. The Unprotect Access Level Is Reconciled Only Where GitLab Reports It

Date: 2026-11-12

## Status

Accepted

## Context

[ADR-005](./0005-a-protected-branch-is-reconciled-by-recreating-it.md) settled that a change to a protected branch is applied by unprotecting the branch and protecting it again.
It also recorded, from a Community Edition instance alone, that the level controlling who may unprotect a branch can be written and never read, and concluded that the level is not something reconciliation has to preserve.
That conclusion did not survive the first Enterprise instance.

An Enterprise build reports `unprotect_access_levels` on every protection whether or not it is licensed.
A create that omits `unprotect_access_level` is given GitLab's default, the Maintainer role.
On GitLab EE 19.3.1 without a licence, a branch whose level was 60 was observed holding 40 after it was unprotected and protected again by a request that did not send the level.
The recreate ADR-005 chose was therefore changing a setting the manifest never mentioned, and the plan showed nothing, which is the silent change ADR-005 was written to rule out.

Declaring the level in the manifest answers that, and raises the opposite problem on the other edition.
A Community Edition build accepts `unprotect_access_level` on create with a `201` and answers every later read without the field.
A level declared against such an instance can never be confirmed: an apply would recreate the branch, the next read would report nothing, and every run would plan the same change again while leaving the branch unprotected for the length of it.

Three further observations bound the decision.

- GitLab refuses an unprotect level of 0, so "no one" cannot be declared.
- The protection GitLab creates together with a project reports an empty unprotect list on an Enterprise build, and a create has no way to ask for an empty list.
- An Enterprise build reports `code_owner_approval_required` and, without a licence, answers `false` to a create that sends `true`.

The last observation concerns what the level does once stored.
GitLab documents the restriction on who may unprotect a branch as a Premium feature.
On GitLab EE 19.5.3 without a licence the stored level decided nothing: a maintainer unprotected a branch whose list held 60, a developer could not unprotect one whose list held 30, and a branch whose list was empty behaved like one holding 40.
GitLab CE 19.5.3 behaved the same way, with the Maintainer role deciding in every case.
Enforcement on a licensed instance was not observed.

The decision has to keep a recreate from changing what the manifest is silent about, has to let one document be pointed at either edition, and may not produce a plan that never settles.

## Alternatives Considered

### Option 1: Leave the level out of the manifest

This is what ADR-005 recorded.
The manifest holds the push and merge roles and the force-push flag, and a recreate sends those alone.

#### Pros

- The manifest means the same thing on every instance.

#### Cons

- On an Enterprise build every access-level change resets the unprotect level to GitLab's default, and neither the plan nor the report says so.

### Option 2: Declare the level and apply it on every instance

The level is a manifest key like the others, compared and sent wherever the manifest is used.

#### Pros

- One rule for every key and every instance.

#### Cons

- On a Community Edition build the declared level is never reported back, so the plan repeats the same update on every run and each apply reopens the branch.
- A document exported from a Community Edition instance has no value to write for the key, so a required key would have to be invented on export.

### Option 3: Declare the level, and reconcile it only where the instance reports it

The level is an optional key.
It is compared and sent on an instance that reports the field, and ignored with a notice on one that does not.

#### Pros

- A recreate on an Enterprise build keeps the level, whether the manifest states it or not.
- A document moves between editions without planning drift that cannot be settled.

#### Cons

- The same document does different things on different instances, and the difference has to be told to the operator.
- The tool has to learn which kind of instance it is talking to.

## Decision

We will take Option 3.

### 1. The level is an optional key, and silence means "leave it as it is"

A manifest may state who may unprotect a branch as `developer`, `maintainer` or `admin`.
A manifest that omits the key plans no change to the level, whatever the instance holds.
An export writes the key only when GitLab reports a role.

#### Rationale

The key exists on one kind of instance and not on the other, so an omitted key cannot be read as a request to clear the level.
Reading it that way would make every document exported from a Community Edition instance plan drift as soon as it was pointed at an Enterprise one.
"No one" is not among the values because GitLab refuses it, and a protection nobody could lift would be permanent.

### 2. A recreate sends the live level when the manifest is silent

When a protection is recreated and the manifest does not state the level, the level the instance reported is sent with the create.

#### Rationale

Without it the create takes GitLab's default, and the recreate becomes the unannounced change of a setting that ADR-005 exists to prevent.
Carrying the live value keeps the recreate an implementation detail of the one change the plan showed.

### 3. A declared level is ignored, and said to be ignored, where the instance does not report the field

On an instance that does not report the field, a declared level is not compared, not shown as a change and not sent.
The plan names the branches whose declared level it is ignoring.

#### Rationale

A value the instance never reports cannot be confirmed, so applying it would plan the same change forever and reopen the branch on every run.
Ignoring a key in silence would leave the operator believing it had been applied, and whether an instance reports the field cannot be known from the document alone, so validation of the document cannot catch it.
The plan is the first place where both the document and the instance are in view.

### 4. Whether the field is reported is asked of the instance, not inferred from its protections

The edition of the instance decides whether the level is reconciled.
A read that cannot learn the edition fails.

#### Rationale

A project that holds no protection yet says nothing about what the instance would report for one, so inferring the edition from the protections would take an Enterprise instance with such a project for a Community one and drop the declared level.
Guessing Community Edition when the edition cannot be learned would do the same.

### 5. The licence is not consulted

On an Enterprise build without a licence the level is reconciled exactly as on a licensed one, although such an instance does not enforce it.

#### Rationale

The stored value is reported, exported and compared whether or not it is enforced, so reconciling it keeps a plan stable and keeps a recreate from changing it.
Refusing the key on an unlicensed instance would make a document stop working when a licence lapses, and would make the tool depend on a second property of the instance for no change in what it writes.

### 6. Code owner approval stays out of the manifest

`code_owner_approval_required` is not a manifest key.

#### Rationale

An Enterprise build without a licence reports the field and refuses the value, so a declaration could be neither applied nor confirmed there, and nothing a recreate does can lose a value the instance will not hold.

### Not Decided Here

- Preserving an empty unprotect list across a recreate: a `PATCH` that destroys the one grant does leave the list empty, as observed on GitLab EE 19.5.3 without a licence, but it needs the identifier of the grant and is the stateful write ADR-005, Option 2 (PATCH the `allowed_to_*` arrays) rejected. It will be decided when an instance is found on which an empty list and the default behave differently.
- Unprotect grants that name a user or a group: this ADR covers role levels alone.

## Consequences

### Positive

- A recreate no longer changes who may unprotect a branch: the level comes back as declared, or as it was when the manifest is silent.
- One document serves both editions: it plans no drift on an instance that cannot report the level.
- An ignored key is visible: the operator is told which branches carry a level the instance cannot confirm.

### Negative

- An Enterprise protection whose unprotect list is empty gains GitLab's default when it is recreated, because that state cannot be told apart from an unstated level.
- On an Enterprise build without a licence a declared level reads as a restriction and restricts nothing.
- A declared level is not applied on a Community Edition instance, and the notice appears only once the document is compared with the instance, never when it is validated.
- Reading a project's protections depends on learning the edition of the instance, so a failure to learn it fails the read.

### Mitigations

- An empty list and the default grant the same role on every instance observed, and the change shows up in a later export.
- The user documentation states that an unlicensed instance stores the level without enforcing it.
- Every plan prints the ignored declarations on standard error until the key is removed or the instance changes.
- The failed read reports its cause, and the rest of the project's settings are still planned.

## References

- [ADR-005](./0005-a-protected-branch-is-reconciled-by-recreating-it.md), whose recreate this decision keeps from changing the level, and whose statement that the level need not be preserved this decision replaces.
- [Protected branches](https://docs.gitlab.com/user/project/repository/branches/protected/), "Control who can unprotect branches".
- [Protected branches API](https://docs.gitlab.com/api/protected_branches/).
