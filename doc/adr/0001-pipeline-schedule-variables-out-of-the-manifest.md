# 1. Pipeline Schedule Variables Stay Out of the Manifest

Date: 2026-08-07

## Status

Accepted

## Context

A GitLab pipeline schedule can carry variables, which the pipeline it starts reads as CI/CD variables.
`infra import` currently exports them into the manifest, values included:

```yaml
  pipeline_schedules:
  - description: nightly
    ref: refs/heads/main
    cron: 0 3 * * *
    variables:
    - key: DEPLOY_TOKEN
      value: glpat-secret-here
      variable_type: env_var
```

The manifest is meant to be committed to version control, so this writes credentials into the repository.
The schema's own comment records the problem rather than solving it: "the manifest carries the value, which makes keeping secrets out of these documents the operator's responsibility."

The value cannot simply be dropped while keeping the rest.
GitLab's API marks `value` as required on both create and update of a schedule variable, so a manifest holding keys alone can never bring a declared variable into existence.

Three further properties of schedule variables, all confirmed against GitLab 19.2.1 CE, make them expensive to manage at all.

Reading them costs one request per schedule.
The list endpoint carries no variables; the observed keys on a list entry are `active, created_at, cron, cron_timezone, description, id, inputs, next_run_at, owner, ref, updated_at`.

Reading them is permission-gated inside a 200 response.
A reader who is neither Maintainer, Owner, nor the schedule's creator receives the schedule with the `variables` field absent rather than an error, as the following observations show.

| reader                           | HTTP | `variables` |
| -------------------------------- | ---- | ----------- |
| instance administrator           | 200  | present     |
| Maintainer, created the schedule | 200  | present     |
| Maintainer, did not create it    | 200  | present     |
| Developer, did not create it     | 200  | **absent**  |

Writing them requires more than reading them.
Every write returned 403 unless the token both owned the schedule and held Maintainer, so a reader who can compare the variables may still be unable to reconcile them.

Doing nothing is not acceptable because the current behaviour puts secrets in version control on the ordinary path: an operator running `infra import` on a project whose schedule carries a token gets that token written to a file they are expected to commit.

The decision has to keep every credential out of version control, because the manifest is committed and anything it holds is disclosed to everyone with repository access.
It has to keep the schema from promising what apply cannot deliver, because a field that can be declared but never realised misleads the reader of the document.
It has to disclose a destructive change before it happens, because deleting a schedule destroys its variables and nothing else in the plan mentions them.
It has to keep the ordinary path cheap, because a per-schedule request multiplies the cost of every plan by the number of schedules a project holds.

## Alternatives Considered

### Option 1: Keys without values

The manifest declares `key` and `variable_type`, never `value`.

```yaml
    variables:
    - key: DEPLOY_TOKEN
      variable_type: env_var
```

#### Pros

- A reader of the manifest learns which variables a schedule carries without opening GitLab.
- Drift in the set of keys is visible, because a variable added by hand appears in the plan as one the manifest does not declare.
- No credential is committed.

#### Cons

- Apply can never create a variable, because GitLab requires `value` on create.
  It can only delete the ones the manifest omits and report the ones it lacks, so a project cannot be reproduced from its manifest.
- Comparing key sets still requires the per-schedule read, so the request cost, the permission-gated absence and the read-versus-write asymmetry all remain.
  Only the credential problem is solved.
- A `variable_type` declared without a value carries almost no information.

### Option 2: No variables anywhere

The manifest has no `variables` key, and no command reads them.

#### Pros

- The per-schedule request disappears entirely, and a project costs one list read.
- The permission-gated absence and the read-versus-write asymmetry stop mattering, because nothing depends on the variables being readable.
- No credential is committed, and the schema promises nothing it cannot deliver.

#### Cons

- Deleting a schedule destroys its variables with no mention of them anywhere, which is the most damaging thing the tool can do silently.
- A reader of the manifest cannot tell that a schedule carries variables at all.

### Option 3: No variables in the manifest, disclosed when a deletion would destroy them

The manifest has no `variables` key.
When `infra plan` finds a live schedule the manifest does not declare, it reads that schedule's variables and names their keys in the plan:

```text
- pipeline_schedule "nightly"
    variables:
      - DEPLOY_TOKEN
```

#### Pros

- It keeps Option 2's cost, because the extra read happens only for a schedule about to be deleted, which is none on the ordinary path.
- It closes the silent destruction, because the operator approving the deletion sees what goes with it.
- A variables read that fails or is refused costs nothing but a warning, because no reconciliation depends on the answer.

#### Cons

- The manifest still says nothing about variables during ordinary reading.
- The plan carries a special case, in which one resource is read only when a deletion is at stake.

## Decision

We will remove pipeline schedule variables from the manifest schema and disclose them only where a planned change would destroy them.

### 1. The schema drops the variables field

A pipeline schedule in the manifest is described by its own attributes alone, and the manifest has no field for its variables.
A manifest that still declares variables is refused as carrying an unknown field rather than having the field ignored.

#### Rationale

A field whose value the document must not hold, and which apply can therefore never create, is a promise the tool cannot keep.
Declaring keys alone would keep every cost of managing variables while removing the only thing that made those costs worth paying.
Refusing a stale field lets an operator learn the field is gone instead of watching it silently stop working.

### 2. `infra import` stops reading and writing variables

The export reads the schedule list alone and writes the schedule's own attributes.
It no longer reads each schedule individually to collect its variables, and no longer writes them, values included, into the emitted document.

#### Rationale

The export is what puts credentials into version control today.
Removing the field from the schema is not enough on its own; the read that produced the values goes with it, which also returns a project's export to a single list request.

### 3. `infra plan` names the variable keys a deletion would destroy

Before a plan renders the deletion of a schedule the manifest does not declare, it reads that schedule's variables and lists their keys under the deletion.
Values are never rendered.
A read that fails or is refused is reported as a warning, and the deletion is still shown.

This read is not the removed per-schedule read.
It answers with keys alone, covers one schedule rather than a project's whole list, and no reconciliation consumes its answer.

There is something to disclose only while plans propose deletions.
[ADR-002](./0002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md) stopped `infra plan` reconciling schedules at all, so no plan proposed a deletion.
[ADR-003](./0003-a-schedule-is-identified-by-its-description.md) restored deletions, and this part of the decision takes effect with that work rather than before it.

#### Rationale

The variables are destroyed with the schedule and nothing else in the plan mentions them, so this is the operator's only chance to see them.
It is scoped to deletions because that is the only change that destroys a variable without naming it, which keeps a plan that deletes nothing at one request per project.
Returning keys rather than variables is what keeps a value from reaching a caller that could render or write it, which is the property the removed read could not offer.

### Not Decided Here

- Migrating manifests already written that still carry `variables`: this record leaves it as an operational question and does not name where it will be answered.
- Values already committed to a repository's history: this record leaves what is done about them as an operational question and does not name where it will be answered.

## Consequences

### Positive

- No credential is written to version control: the ordinary `infra import` path no longer emits values, so the file an operator commits cannot carry a token that a schedule holds.
- A project costs one request per read: removing the per-schedule read removed the multiplication by schedule count.
  A plan pays one disclosure read per schedule it removes, and none otherwise.
  `infra apply` adds no read of its own, because the disclosure happens while the plan it then performs is built.
- Three failure modes are confined to one read: the permission-gated absence of the variables field, the mismatch between who may read and who may write them, and the distinction between "not read" and "read, none" all left the reconciliation.
  Decision 3 brings them back only into the disclosure read, where nothing compares against the answer, so getting one wrong costs a warning rather than a plan.
- The document stops overstating itself: a manifest no longer looks as though it describes a schedule's variables when applying it could not produce them.

### Negative

- Variables are managed outside this tool: an operator declaring a schedule still has to create its variables by hand or with something else, and the manifest gives no sign that this is pending.
- Drift in variables is invisible: a variable added, changed or removed by hand never appears in a plan, so the manifest and the project can disagree without anyone noticing.
- The plan carries a conditional read: one path reads a resource only when a removal is planned, which is a special case a reader has to learn.
- Removing the field breaks manifests already written: a document that still carries `variables` stops being accepted.

### Mitigations

- The user documentation says that schedule variables are outside the manifest, so an operator is not left inferring it from a field that has quietly stopped being accepted.
- A plan that deletes a schedule names the keys of the variables the deletion destroys, so the destructive case is never silent.
  The drift that stays invisible changes what a pipeline receives and destroys nothing.
- The conditional read is named for what it is and justified where it happens, so it reads as a disclosure step rather than as part of reconciliation.
- A manifest still carrying `variables` is refused as carrying an unknown field rather than being ignored, so an operator learns the field is gone.

## References

- [ADR-002](./0002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md)
- GitLab pipeline schedules API, which marks `value` required on both create and update of a schedule variable, and states that `variables` is included in a response only for Maintainer, Owner, or the schedule's owner.
