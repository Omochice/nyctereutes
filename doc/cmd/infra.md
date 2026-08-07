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

`infra import` exports a project's pipeline schedules, `infra validate` checks them, and `infra plan` reports how the schedules a project holds differ from the ones a manifest declares.

Writing that difference back is not in place yet. `infra apply` reports a schedule change as unsupported instead of performing it, and counts it as a failure, so a plan showing schedule drift can be read but not carried out and nothing is applied by halves.

A schedule is identified by its description. GitLab addresses one by a server-assigned id that no manifest holds, so the description is what pairs a declared schedule with a live one. GitLab does not require descriptions to be unique, and a project holding two schedules described alike is refused, because a change addressed to that description would land on an arbitrary member of the pair. The refusal names both ids, and it costs the project its schedules rather than the project: its other settings are still planned.

Because the description is the identity, editing one is not a rename. A plan reports the schedule with the old description as removed and one with the new description as created, so it would lose its id and everything GitLab keeps against that id.

The `pipeline_schedules` key follows the same rule as `topics`. A manifest that omits the key says nothing about schedules and no plan reports one. A manifest that carries the key with an empty list declares that the project holds no schedule, so a plan reports every one that exists as a removal. This matters for an exported manifest: `infra import` writes `pipeline_schedules: []` for a project owning none, so committing that document is a declaration that none may be added.

An omitted attribute takes the value GitLab defaults on create, which for `active` is `true`. A declared schedule that does not say `active: false` is therefore a declaration that it runs, and a plan reports a schedule someone paused in the web UI as one to resume.

A schedule's variables are outside the manifest. They are not exported, and a manifest declaring a `variables` block on a schedule is rejected as carrying an unknown field rather than being read and ignored, so a schedule in a manifest describes when a pipeline runs and not what it receives. GitLab requires a value to create a variable, and a manifest that is committed to version control is the wrong place to keep one, so a declared variable could never be created from the document that declares it.

## Inspired by

This command is inspired by [babarot/gh-infra](https://github.com/babarot/gh-infra).
