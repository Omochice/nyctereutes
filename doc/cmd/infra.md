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

A project's pipeline schedules are managed the way its own settings are: `infra import` exports them, `infra validate` checks them, and `infra plan` and `infra apply` create, change and remove them so the project matches what the manifest declares.

A schedule is identified by its description. GitLab addresses one by a server-assigned id that no manifest holds, so the description is what pairs a declared schedule with a live one. GitLab does not require descriptions to be unique, and a project holding two schedules described alike is refused by import, plan and apply, because a change addressed to that description would land on an arbitrary member of the pair. The refusal names both ids, and it costs the project its schedules rather than the project: its other settings are still planned and applied.

Because the description is the identity, editing one is not a rename. The schedule with the old description is removed and a schedule with the new one is created, so it loses its id and everything GitLab keeps against that id.

The `pipeline_schedules` key follows the same rule as `topics`. A manifest that omits the key says nothing about schedules and leaves every live one alone. A manifest that carries the key with an empty list declares that the project holds no schedule, so applying it removes every one that exists. This matters for an exported manifest: `infra import` writes `pipeline_schedules: []` for a project owning none, so committing that document is a declaration that none may be added.

An omitted attribute takes the value GitLab defaults on create, which for `active` is `true`. A declared schedule that does not say `active: false` is therefore a declaration that it runs, and applying such a manifest resumes a schedule someone paused in the web UI.

A schedule's variables are outside the manifest. They are not exported, and a manifest declaring a `variables` block on a schedule is rejected as carrying an unknown field rather than being read and ignored, so a schedule in a manifest describes when a pipeline runs and not what it receives. GitLab requires a value to create a variable, and a manifest that is committed to version control is the wrong place to keep one, so a declared variable could never be created from the document that declares it.

Removing a schedule destroys its variables, and nothing else in a plan would mention them, so a planned removal lists the keys of the variables going with it. Only their keys are read, never their values. GitLab answers a variables read with the field left out for a reader who is neither Maintainer, Owner, nor the schedule's creator; that is reported as a warning and the removal is still shown, because performing it does not depend on the answer.

Writing a schedule needs more than reading one. Reading a project's schedules needs only the access that reads the project, while every write is refused unless the token both holds the Maintainer or Owner role and belongs to the user who created that schedule. A plan can therefore be correct and its apply refused, which the apply error says explicitly rather than blaming the role alone.

## Inspired by

This command is inspired by [babarot/gh-infra](https://github.com/babarot/gh-infra).
