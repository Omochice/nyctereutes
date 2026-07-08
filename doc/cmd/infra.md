# infra

`infra` manages GitLab project settings declaratively through YAML manifests.

It follows an import, validate, plan, apply cycle so the manifests stay the single source of truth for project settings, and every change to live GitLab state is previewed before it is made.

## Subcommands

`infra import` exports the settings of existing GitLab projects as YAML manifests.
`infra validate` validates manifest files against the schema.
`infra plan` shows the drift between manifests and live GitLab state.
`infra apply` applies manifests to live GitLab state after a confirmation prompt.
