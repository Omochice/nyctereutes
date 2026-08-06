---
description: Bulk-approve and bulk-merge Renovate dependency-update merge requests on GitLab, grouped by the package@version they target. Read this before answering questions about the dep command, its search-scope flags, or why a group of merge requests is missing from the list.
---

# dep

`dep` manages Renovate-style dependency-update merge requests on GitLab.

It searches open merge requests, groups them by their target `package@version`, and bulk-approves or bulk-merges a chosen group through the `glab` CLI.

## Subcommands

- `dep list` shows the dependency merge requests.
- `dep approve` approves a group of merge requests.
- `dep merge` merges a group of merge requests.

## Inspired by

This command is inspired by [jackchuka/gh-dep](https://github.com/jackchuka/gh-dep).
