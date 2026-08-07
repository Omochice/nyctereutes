---
description: Manage GitLab project settings declaratively through YAML manifests, following an import, validate, plan, apply cycle. Read this before answering questions about the infra command, the manifest schema, or a plan that reports drift nobody expected.
---

# infra

`infra` manages GitLab project settings declaratively through YAML manifests.

It follows an import, validate, plan, apply cycle so the manifests stay the single source of truth for project settings, and every change to live GitLab state is previewed before it is made.

## Subcommands

- `infra import` exports the settings of existing GitLab projects as YAML manifests.
- `infra validate` validates manifest files against the schema.
- `infra plan` shows the drift between manifests and live GitLab state.
- `infra apply` applies manifests to live GitLab state after a confirmation prompt.

## Pipeline schedules

`infra import` exports a project's pipeline schedules, and `infra validate` checks them, but `infra plan` and `infra apply` do not reconcile them: a declared schedule is neither created, changed nor removed, and a plan reports no drift in one.

A schedule is exported so a manifest describes the project as it stands, not so the manifest drives it. Reconciling one needs the live set to be knowable, and it is not: GitLab addresses a schedule by an id no manifest can hold, so a declared one is paired by its description, which GitLab neither keeps unique nor keeps stable when someone edits it in the web UI. Reading a schedule and writing it also need different permissions, so a plan can be correct and its apply refused.

Whether schedules become manageable, and under what model, is an open question rather than a planned feature. See `doc/adr/adr-002-pipeline-schedules-are-not-reconciled-as-a-declared-set.md`.

## Inspired by

This command is inspired by [babarot/gh-infra](https://github.com/babarot/gh-infra).
