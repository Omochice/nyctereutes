# ADR-001: Pipeline Schedule Variables Stay Out of the Manifest

## Status

Proposed

## Context

A GitLab pipeline schedule can carry variables, which the pipeline it starts
reads as CI/CD variables. `infra import` currently exports them into the
manifest, values included:

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

The manifest is meant to be committed to version control, so this writes
credentials into the repository. The schema's own comment records the problem
rather than solving it: "the manifest carries the value, which makes keeping
secrets out of these documents the operator's responsibility."

The value cannot simply be dropped while keeping the rest. GitLab's API marks
`value` as required on both create and update of a schedule variable, so a
manifest holding keys alone can never bring a declared variable into existence.

Three further properties of schedule variables, all confirmed against GitLab
19.2.1 CE, make them expensive to manage at all:

Reading them costs one request per schedule. The list endpoint carries no
variables; the observed keys on a list entry are `active, created_at, cron,
cron_timezone, description, id, inputs, next_run_at, owner, ref, updated_at`.

Reading them is permission-gated inside a 200 response. A reader who is neither
Maintainer, Owner, nor the schedule's creator receives the schedule with the
`variables` field absent rather than an error:

| reader                           | HTTP | `variables` |
| -------------------------------- | ---- | ----------- |
| instance administrator           | 200  | present     |
| Maintainer, created the schedule | 200  | present     |
| Maintainer, did not create it    | 200  | present     |
| Developer, did not create it     | 200  | **absent**  |

Writing them requires more than reading them. Every write returned 403 unless
the token both owned the schedule and held Maintainer, so a reader who can
compare the variables may still be unable to reconcile them.

Doing nothing is not acceptable because the current behaviour puts secrets in
version control on the ordinary path: an operator running `infra import` on a
project whose schedule carries a token gets that token written to a file they
are expected to commit.

## Decision Drivers

- **No credential reaches version control**: the manifest is committed, so
  anything it holds is disclosed to everyone with repository access.
- **The schema does not promise what apply cannot deliver**: a field that can be
  declared but never realised misleads the reader of the document.
- **A destructive change is disclosed before it happens**: deleting a schedule
  destroys its variables, and nothing else in the plan mentions them.
- **The ordinary path stays cheap**: a per-schedule request multiplies the cost
  of every plan by the number of schedules a project holds.

## Considered Options

### Option 1: Keys without values

The manifest declares `key` and `variable_type`, never `value`.

```yaml
    variables:
    - key: DEPLOY_TOKEN
      variable_type: env_var
```

**Pros:**

- A reader of the manifest learns which variables a schedule carries without
  opening GitLab.
- Drift in the set of keys is visible: a variable added by hand appears in the
  plan as one the manifest does not declare.
- No credential is committed.

**Cons:**

- Apply can never create a variable, because GitLab requires `value` on create.
  It can only delete the ones the manifest omits and report the ones it lacks,
  so a project cannot be reproduced from its manifest.
- Comparing key sets still requires the per-schedule read, so the request cost,
  the permission-gated absence and the read-versus-write asymmetry all remain.
  Only the credential problem is solved.
- `variable_type` declared without a value carries almost no information.

### Option 2: No variables anywhere

The manifest has no `variables` key, and no command reads them.

**Pros:**

- The per-schedule request disappears entirely; a project costs one list read.
- The permission-gated absence and the read-versus-write asymmetry stop
  mattering, because nothing depends on the variables being readable.
- No credential is committed, and the schema promises nothing it cannot deliver.

**Cons:**

- Deleting a schedule destroys its variables with no mention of them anywhere,
  which is the most damaging thing the tool can do silently.
- A reader of the manifest cannot tell that a schedule carries variables at all.

### Option 3: No variables in the manifest, disclosed when a deletion would destroy them

The manifest has no `variables` key. When `infra plan` finds a live schedule the
manifest does not declare, it reads that schedule's variables and names their
keys in the plan:

```text
- pipeline_schedule "nightly"
    variables:
      - DEPLOY_TOKEN
```

**Pros:**

- Keeps Option 2's cost: the extra read happens only for a schedule about to be
  deleted, which is none on the ordinary path.
- Closes the silent destruction: the operator approving the deletion sees what
  goes with it.
- A variables read that fails or is refused costs nothing but a warning, because
  no reconciliation depends on the answer.

**Cons:**

- The manifest still says nothing about variables during ordinary reading.
- The plan carries a special case: one resource read only when a deletion is at
  stake.

## Decision

We will remove pipeline schedule variables from the manifest schema and disclose
them only where a planned change would destroy them.

### 1. The schema drops the variables field

**Change from**:

```go
type PipelineSchedule struct {
	Description  string       `yaml:"description"`
	Ref          Ref          `yaml:"ref"`
	Cron         string       `yaml:"cron"`
	CronTimezone string       `yaml:"cron_timezone"`
	Active       bool         `yaml:"active"`
	Variables    []ScheduleVariable `yaml:"variables,omitzero"`
}
```

**Change to**:

```go
type PipelineSchedule struct {
	Description  string `yaml:"description"`
	Ref          Ref    `yaml:"ref"`
	Cron         string `yaml:"cron"`
	CronTimezone string `yaml:"cron_timezone"`
	Active       bool   `yaml:"active"`
}
```

**Rationale**: A field whose value the document must not hold, and which apply
can therefore never create, is a promise the tool cannot keep. Declaring keys
alone would keep every cost of managing variables while removing the only thing
that made those costs worth paying.

### 2. `infra import` stops reading and writing variables

**Change from**: the export reads each schedule individually to collect its
variables and writes them, values included, into the emitted document.

**Change to**: the export reads the schedule list alone and writes the
schedule's own attributes.

**Rationale**: The export is what puts credentials into version control today.
Removing the field from the schema is not enough on its own; the read that
produced the values goes with it, which also returns a project's export to a
single list request.

### 3. `infra plan` names the variable keys a deletion would destroy

**Change from**: a schedule the manifest does not declare renders as one line
naming the schedule.

**Change to**: before rendering such a deletion, the schedule's variables are
read and their keys are listed under it. Values are never rendered. A read that
fails or is refused is reported as a warning and the deletion is still shown.

**Rationale**: The variables are destroyed with the schedule and nothing else in
the plan mentions them, so this is the operator's only chance to see them. It is
scoped to deletions because that is the only change that destroys a variable
without naming it, which keeps the ordinary plan at one request per project.

## Consequences

### Positive

1. **No credential is written to version control**: the ordinary `infra import`
   path stops emitting values, so the file an operator commits cannot carry a
   token that a schedule holds.
2. **A project's plan costs one request again**: removing the per-schedule read
   removes the multiplication by schedule count on every plan and apply.
3. **Three failure modes stop existing**: the permission-gated absence of the
   variables field, the mismatch between who may read and who may write them,
   and the distinction between "not read" and "read, none" all stop being
   states the code has to represent.
4. **The document stops overstating itself**: a manifest no longer looks as
   though it describes a schedule's variables when applying it could not produce
   them.

### Negative

1. **Variables are managed outside this tool**: an operator declaring a schedule
   still has to create its variables by hand or with something else, and the
   manifest gives no sign that this is pending.
2. **Drift in variables is invisible**: a variable added, changed or removed by
   hand never appears in a plan, so the manifest and the project can disagree
   without anyone noticing.
3. **The plan gains a conditional read**: one code path reads a resource only
   when a deletion is planned, which is a special case a reader has to learn.

### Mitigations

- The command documentation states that schedule variables are outside the
  manifest, so an operator is not left inferring it from a missing field.
- The deletion disclosure covers the case where invisible drift is destructive;
  the remaining invisible drift changes what a pipeline receives but destroys
  nothing.
- The conditional read is named for what it is and justified where it happens,
  so it reads as a disclosure step rather than as part of reconciliation.

## Implementation Notes

This reverts part of what is already on `main`. The `variables` block, the
per-schedule read that fills it and the export that writes it were merged in
PR #68, and they come back out: the schema type, `Client.FetchSchedules`'s
`withVariables` parameter, `fetchScheduleVariables` and the fake support behind
them. The schedule's own attributes stay, so the revert is partial rather than a
reversal of the whole feature.

Removing a schema field is a breaking change for any manifest already written.
A document still carrying `variables` reports it as an unknown field rather than
being ignored, which the schema's strict decoding already does, so an operator
learns the field is gone instead of watching it silently stop working.

Verification worth pinning: an export of a project whose schedule carries a
variable contains neither the key nor the value; a plan deleting such a schedule
names the key and not the value; a plan that deletes nothing issues one request
per project.

## References

- [ADR-002](./adr-002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md)
- GitLab pipeline schedules API, which marks `value` required on both create and
  update of a schedule variable, and states that `variables` is included in a
  response only for Maintainer, Owner, or the schedule's owner.
