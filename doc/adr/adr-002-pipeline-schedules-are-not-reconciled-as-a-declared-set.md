# ADR-002: Stop Reconciling Pipeline Schedules as a Declared Set

## Status

Accepted. The question decision 2 deferred is answered by [ADR-003](./adr-003-a-schedule-is-identified-by-its-description.md).

## Context

`infra plan` and `infra apply` treat a manifest as the complete desired state: what it declares is made true, and what it omits is removed. This works for a project's own settings, and PR #66 extended it to pipeline schedules, pairing a declared schedule with a live one by description and creating, updating or deleting to match.

The branch did not converge. Nine commits of review fixes produced the same class of defect repeatedly, and each fix was a new way to say "this is not known":

| what was patched                              | the state that had to be added                   |
| --------------------------------------------- | ------------------------------------------------ |
| `pipeline_schedules` nil versus `[]`          | the manifest declares nothing about them         |
| `variables` nil versus `[]`                   | the schedule declares nothing about them         |
| the JSON `variables` field absent versus `[]` | the token may not see them                       |
| `SchedulesMissingVariables`                   | this schedule's variables went unread            |
| `withoutSchedules`                            | this run could not read them, so it manages none |

Three guards were also reversed more than once. The refusal to plan against unreadable variables was added, then narrowed, and is still contested. The rule abandoning changes after a failed delete was applied to every schedule change, then narrowed to creates, then removed entirely. The condition deciding whether to read variables was gated on the manifest declaring some, then made unconditional.

A review of the whole branch then reported eight further correctness findings, several of them created by those same guards.

The recurring shape is not sloppiness in any one fix. Reconciling a declared set requires knowing the live set, and three properties of pipeline schedules, all confirmed against GitLab 19.2.1 CE, mean it is only partly knowable:

A schedule has no identity the manifest can hold. GitLab addresses it by a server-assigned id, so the manifest pairs by description instead. GitLab does not enforce uniqueness of descriptions, which already forced the tool to refuse any project holding two schedules described alike. Renaming a description in the web UI reads here as a deletion and a creation.

Reading a schedule and writing it need different permissions. Every write returned 403 unless the token both owned the schedule and held Maintainer, while a Maintainer who did not create it can read it. A plan can therefore be correct and its apply refused.

The schedule's variables are permission-gated within a successful response, and carry credentials. [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) takes them out of the manifest for that reason. Once it is carried out, the parts of PR #66 that diff and apply manifest-held variables have nothing left to reconcile, and those are most of the branch.

Doing nothing is not acceptable because the branch's current behaviour is destructive on partial knowledge. A live state that was never read reports every declared schedule as missing, GitLab accepts a second schedule described like the first, and the manifest can no longer tell the copies apart.

## Decision

We will discard PR #66 and stop treating pipeline schedules as a declared set that `infra apply` reconciles.

### 1. PR #66 is closed and its branch discarded

**Change from**: nine commits on `feat/pipeline-schedules` adding schedule and variable diffing, rendering and applying, plus the guards added during review.

**Change to**: the branch is not merged. What was learned from it is recorded in this file and in [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md), which is where a reader without that branch checked out has to be able to find it.

**Rationale**: The branch is built on the declared-set model this ADR rejects and on the variables schema [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) removes, so most of it would be rewritten rather than amended. Continuing to patch it spends review on defects the model keeps producing.

### 2. The reconciliation model for schedules is left undecided

**Change from**: schedules are reconciled as a declared set, deletions included.

**Change to**: `infra plan` and `infra apply` manage a project's own settings only. Whether schedules are reconciled at all, and under what model if so, is deferred to a later ADR.

**Rationale**: The evidence gathered so far says the declared-set model does not fit, but it does not by itself say which model does. Recording the stop separately from the replacement keeps this decision reviewable on what it actually rests on, and stops a replacement being chosen under the pressure of an open branch.

Whatever settles that question inherits one obligation from here. Stopping deletions is what makes decision 3 of [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) unnecessary today: no plan destroys a schedule, so none has to name the variables going with it. A model that deletes again has to carry that disclosure with it, or it restores the silent destruction ADR-001 was written to prevent.

## Consequences

### Positive

1. **The destructive path is closed**: manifest reconciliation neither deletes nor creates a schedule, so partial knowledge of the live state cannot cost an operator a schedule or the variables attached to it. This covers the path this record governs; a schedule remains removable through GitLab itself and through anything else holding the same token.
2. **Review stops paying for a model that does not hold**: the findings still open on PR #66 do not need answering, because the code they describe is not going in.
3. **The next attempt starts from what was measured**: the permission matrix, the response shapes and the identity problem are written down rather than rediscovered.

### Negative

1. **A declared schedule is not applied**: an operator writing `pipeline_schedules` in a manifest gets no reconciliation from it, and the import side of the feature is left describing something the tool will not act on.
2. **Work is thrown away, some of it already merged**: nine commits on the branch, including the schedule diffing and rendering that were not themselves wrong, are discarded rather than reused, and parts of PR #67 and PR #68 are reverted from `main` on top of that.
3. **The gap is open-ended**: deferring the replacement means there is no date by which schedules become manageable, and the question can go stale.

### Mitigations

- `doc/cmd/infra.md` gains a section saying schedules are exported and validated but not reconciled, and why, so the manifest does not silently imply more than the tool does. It ships with this decision rather than after it, because a manifest declaring schedules that nothing applies is already the state on `main` once PR #66 is closed.
- The discarded branch stays reachable in the repository's history and its findings are summarised here, so a later attempt can lift the parts that were sound.
- The deferred question is recorded as a follow-up ADR to be written, not as an informal intention.

## Implementation Notes

Discarding PR #66 is not the whole of it. Reverting part of what is already on `main` is in scope for carrying this decision out, and [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md) does exactly that for the variables.

For the schedules themselves, `infra import` keeps exporting them and `infra validate` keeps checking the block, so an exported document still round trips. Whether an exported block that nothing applies earns its place, or whether the export comes out too, belongs to the deferred decision rather than to this one. Nothing here depends on keeping it: if the later ADR removes the export, that is a further partial revert of PR #67 and PR #68 and not a reversal of this decision.

## References

- [ADR-001](./adr-001-pipeline-schedule-variables-out-of-the-manifest.md)
- PR #66, and the review findings that prompted this decision
- An ordering defect found during that review, independent of this decision and predating the branch: `infra apply` archives a project before writing the settings that follow it, and GitLab makes an archived project read-only, so every later write is refused and re-running cannot repair it.
