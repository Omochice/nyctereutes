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

## Inspired by

This command is inspired by [babarot/gh-infra](https://github.com/babarot/gh-infra).
