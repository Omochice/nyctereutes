# dep

The `dep` command manages Renovate-style dependency-update merge requests on
GitLab by driving the `glab` CLI. It searches open MRs, groups them by the
target `package@version`, and bulk-approves or bulk-merges a chosen group.

Running `dep` with no subcommand is reserved for a future interactive TUI and
currently reports `not implemented`.

## Subcommands

The command exposes three subcommands, described below.

- `dep list` lists dependency MRs, optionally grouped or as JSON.
- `dep approve` bulk-approves every MR in a group.
- `dep merge` bulk-merges every MR in a group.

## list

`dep list` searches for dependency MRs and prints them. Without `--group` it
prints a flat `PROJECT / MR / TITLE` table; with `--group` it prints MRs bucketed
by `package@version`, groups sorted alphabetically.

- `--group` groups MRs by `package@version`.
- `--json` emits the result as JSON instead of a table.

## approve

`dep approve --group <package@version>` approves every MR in the named group.
A repeated approval that GitLab answers with `401` is treated as success, so an
already-approved MR does not abort the run; other failures are reported per MR
and the run continues.

- `--group` selects the target group and is required.
- `--dry-run` prints the intended actions without calling GitLab.

## merge

`dep merge --group <package@version>` merges every MR in the named group. By
default it uses GitLab's native auto-merge, which merges each MR once its
pipeline succeeds.

- `--group` selects the target group and is required.
- `--method` is the merge method (`merge`, `squash`, or `rebase`); the default is
    `squash`.
- `--require-checks` gates the merge on a passing pipeline and defaults to true;
    pass `--require-checks=false` to merge immediately.
- `--dry-run` prints the intended actions without calling GitLab.

## Search scope

`list`, `approve`, and `merge` share the same flags for choosing which MRs to
search. When a flag is omitted, the value falls back to `glab config get dep.*`
and then to a built-in default.

- `--repo`, `-R` targets explicit projects (`GROUP/PROJECT`, comma-separated);
    defaults to `dep.repo`.
- `--author` filters by author; defaults to `dep.author`, then to the Renovate
    bot username `renovate-bot`.
- `--label` filters by MR label.
- `--group-path` targets a GitLab group or subgroup full path.
- `--reviewer` filters by reviewer username.
- `--limit` caps how many MRs are fetched per author across the targeted scope
    (default 200).

The `package@version` grouping is recomputed on every `approve` and `merge`
invocation rather than read from a cache, so the selected group always reflects
the current state of GitLab.

## Inspired by

This command is inspired by [jackchuka/gh-dep](https://github.com/jackchuka/gh-dep),
reimplemented for GitLab and this project's command design.
