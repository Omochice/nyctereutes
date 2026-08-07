# ADR-003: A Pipeline Schedule Is Identified by Its Description

## Status

Proposed

## Context

[ADR-002](./adr-002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md) stopped reconciling pipeline schedules and left open whether they would be reconciled again, and under what model. `infra import` exports them and `infra validate` checks them, but `infra plan` and `infra apply` ignore them, so a manifest can declare a schedule that nothing acts on. That is a state to pass through, not one to settle in: the schema invites a declaration the tool then disregards.

ADR-002 gave three reasons the declared-set model did not hold. Two have since changed.

The variables are gone. [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) took them out of the manifest, which removed the per-schedule request, the field GitLab omits by permission inside a 200 response, and the credentials the document was carrying. Most of what made the earlier attempt hard to reason about went with them.

The identity problem has an answer that was already implemented. GitLab addresses a schedule by a server-assigned id no manifest can hold, so a declared schedule is paired with a live one by description, and GitLab does not enforce that descriptions are unique. `rejectDuplicateDescriptions` already refuses such a project, naming both ids so the reader knows which to rename:

```text
read pipeline schedules group/proj: duplicate pipeline schedule description: "nightly" is used by schedules 1 and 5
```

What was missing was not the check but the decision behind it. Treating that refusal as the tool's rule, rather than as a defensive measure, makes the description an identity the tool requires rather than one it hopes for.

The third reason stands. Writing a schedule needs the token to own it and hold Maintainer, while a Maintainer who did not create it can read it, both confirmed against GitLab 19.2.1 CE. A plan can be correct and its apply refused with 403.

One hazard is not addressed by requiring unique descriptions. Editing a description in the web UI reads here as one schedule disappearing and another appearing, so a reconciliation deletes and recreates rather than renames. The check cannot catch it, because nothing is duplicated at any point.

## Decision Drivers

- **A declared schedule is acted on**: a field the schema accepts and the tool ignores misleads whoever writes it.
- **Pairing rests on something stated, not assumed**: the manifest holds no id, so whatever stands in for one has to be a rule the tool enforces.
- **A project the tool cannot describe is refused, not guessed at**: an ambiguous pairing would put an update or a delete on an arbitrary schedule.
- **One unreconcilable child resource does not hide a project's other drift**: the settings a run can compare are still reported.

## Considered Options

### Option 1: Leave the export in place and never reconcile

`infra import` keeps writing `pipeline_schedules`; `infra plan` and `infra apply` keep ignoring it, and the documentation says so permanently.

**Pros:**

- Nothing can be destroyed by a wrong pairing, because nothing is written.
- No new failure mode reaches an operator.

**Cons:**

- The schema keeps a block that means nothing on apply, which is the state this record exists to leave.
- A project's schedules cannot be brought back from a manifest, so the document is a description rather than a source.

### Option 2: Reconcile only the schedules the manifest names

A declared schedule is created or updated; a live one the manifest does not name is left alone.

**Pros:**

- Nothing is ever deleted, so a wrong pairing costs an update rather than a schedule.
- Partial knowledge of the live set stops being dangerous.

**Cons:**

- A schedule removed from the manifest stays live forever, so the document cannot express "this project runs these and nothing else".
- Topics and the project settings are reconciled as complete sets, so one child resource would follow a different rule for reasons the reader has to look up.

### Option 3: Reconcile as a declared set, with the description required to be unique

A declared list is the complete desired set. A live schedule the manifest does not declare is deleted. A project holding two schedules described alike is refused before any of that.

**Pros:**

- The manifest means for schedules what it already means for every other setting.
- The refusal is already implemented and already reports both ids.
- Requiring uniqueness makes the description an identity rather than a guess, which is what a delete has to rest on.

**Cons:**

- Editing a description in the web UI reads as a deletion and a creation, so the schedule is recreated with a new id.
- A token that may read a schedule but does not own it produces a correct plan and a refused apply.

## Decision

We will reconcile pipeline schedules as a declared set, and require the description to identify one.

### 1. The description is the identity, and a project that does not respect it is refused

**Change from**: `rejectDuplicateDescriptions` refuses a project during `infra import` as a guard against an ambiguous pairing.

**Change to**: the same refusal covers `infra plan` and `infra apply`, and is stated as the rule the manifest relies on: within a project, a pipeline schedule is identified by its description, and a project holding two alike cannot be managed until one is renamed.

**Rationale**: A delete has to rest on knowing which live schedule a declaration refers to. GitLab offers no identity a document can hold, so the tool supplies one by requiring it. Refusing the ambiguous case is what makes the rest safe; without it the check would be a guard against a situation the tool otherwise tolerates.

### 2. A declared list is the complete desired set

**Change from**: `infra plan` and `infra apply` ignore `pipeline_schedules` entirely.

**Change to**: a declared schedule with no live counterpart is created, one that differs is updated, and a live schedule the declaration omits is deleted. A manifest carrying no `pipeline_schedules` key at all says nothing about schedules, so every live one is left alone. A manifest carrying the key with an empty list declares that the project holds no schedule, so every live one is deleted.

**Rationale**: This is what the manifest already means for topics and for the project's own settings. A child resource following a different rule would need the reader to learn which fields are total and which are additive, and the three states the schema already distinguishes exist to express exactly this.

### 3. Renaming a description recreates the schedule

**Change from**: unstated.

**Change to**: the documentation says that changing a schedule's description, in the manifest or in the web UI, is read as removing one schedule and adding another, so the schedule is recreated with a new id.

**Rationale**: The alternative is to guess that two schedules differing in description are the same one, which is the ambiguity decision 1 refuses everywhere else. Stating it makes the description something an operator knows to keep stable, which is the same contract a declarative tool asks for wherever a name stands in for an identity.

## Consequences

This record is proposed, so what follows describes the state once it is carried out rather than how the tool behaves today. Today `infra plan` and `infra apply` still ignore schedules, which is [ADR-002](./adr-002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md)'s decision, and the one thing below that already exists is named as such.

### Positive

1. **A declared schedule will be acted on**: the block the schema accepts becomes the block apply reconciles, so writing one has an effect and reading a manifest tells the truth about the project.
2. **The pairing will rest on a stated rule**: a delete addresses the schedule a declaration names, because a project where that is ambiguous is refused before any change is planned.
3. **The refusal already exists and already explains itself**: `infra import` names both ids today, so the message plan and apply will reuse is one the operator can already act on rather than one being invented with them.
4. **The model will match the rest of the manifest**: a reader who knows what an omitted topic means will know what an omitted schedule means.

### Negative

1. **A rename will destroy and recreate**: editing a description costs the schedule its id, and with it whatever GitLab associates with that id, for a change an operator will read as cosmetic.
2. **A correct plan can become a refused apply**: a token that reads a schedule it does not own produces a plan it cannot carry out, and 403 is what the operator sees.
3. **A deletion will destroy variables the manifest never held**: [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) took variables out of the document, so a deleted schedule takes variables nothing in the plan mentions.
4. **A duplicate description will block a project's schedules entirely**: one pair created by two different people leaves the schedules unmanageable until someone renames one.

### Mitigations

Each of these is written with the reconciliation rather than before it, because a mitigation for behaviour that does not exist has nothing to attach to.

- The documentation gains the rename behaviour, so an operator meets the cost in `doc/cmd/infra.md` rather than in a plan that deletes a schedule they meant to rename.
- The apply error distinguishes a missing role from a schedule owned by someone else, because those have different fixes and the generic permission hint names the wrong one.
- Decision 3 of [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) lands with this: a plan deleting a schedule names the variable keys going with it. That decision was deferred only because nothing deleted a schedule, and carrying this record out is what changes that.
- A project refused for duplicate descriptions still has its other settings planned and applied, so the ambiguity costs its schedules rather than the project.

## Implementation Notes

The reconciliation is written fresh rather than lifted from the discarded branch. Roughly half of that branch was variable diffing and applying, which [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) leaves nothing to reconcile, and the guards it accumulated were answers to problems that came from the variables.

Two defects found while reviewing that branch apply to whatever is written here, and neither is caused by this decision. `infra apply` archives a project before writing the settings that follow it, and GitLab makes an archived project read-only, so a manifest that both archives and changes schedules fails on every later write. Separately, a schedule GitLab reports without an id would have an update or a delete addressed to `pipeline_schedules/0`.

Verification worth pinning: a live schedule the manifest omits is planned for deletion and its variable keys are named; a project holding two schedules described alike is refused by plan and apply as it already is by import, with the other settings still reported; a manifest declaring no schedules plans no schedule change; an empty list plans the removal of every live schedule.

## References

- [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md), whose third decision this record makes due
- [ADR-002](./adr-002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md), which deferred the question this record answers
- PR #66, the discarded attempt, and the review findings it produced
