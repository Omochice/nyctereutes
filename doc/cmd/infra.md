---
description: Manage GitLab project settings declaratively through YAML manifests, following an import, validate, plan, apply cycle. Read this before answering questions about the infra command, the manifest schema, the pipeline schedules or protected branches it manages, or a plan that reports drift nobody expected.
---

# infra

`infra` manages GitLab project settings declaratively through YAML manifests.

It follows an import, validate, plan, apply cycle so the manifests stay the single source of truth for project settings, and every change to live GitLab state is previewed before it is made.

## Subcommands

- `infra import` exports the settings of existing GitLab projects as YAML manifests.
- `infra validate` validates manifest files against the schema.
- `infra plan` shows the drift between manifests and live GitLab state.
- `infra apply` applies manifests to live GitLab state after a confirmation prompt.

## The manifest schema

An exported manifest is meant to be hand-edited, so `infra import` heads every document it writes with a `yaml-language-server` modeline naming the JSON Schema this repository publishes. The line is repeated after each `---`, because an editor reads it out of a single document's own leading comments and does not carry it across a separator. An editor that understands the convention validates and completes the document as it is written: it offers the keys a manifest may hold and the values an enum accepts, and reports an unknown key, a wrong type or a missing required field before any command is run.

The URL names the schema as it stood in the sources the exporting build was made from. A build that knows its own revision names that revision, a release names its tag, and a build that knows neither names `refs/heads/main`, so an editor applies the rules of the code that produced the export rather than whatever has been committed since. Nothing rewrites the line afterwards, so a manifest kept across releases keeps naming the sources it was exported from until someone imports it again.

The schema is derived from the same Go types the commands parse with, so the rules an editor applies and the rules a command applies cannot drift apart. It is a weaker check rather than a replacement for `infra validate`: a rule relating one part of a document to another has no expression in JSON Schema, so a manifest declaring two schedules with the same description passes an editor unremarked and is still refused when it is validated. It is stricter about one thing, because every optional field is typed concretely and an explicit null therefore reads as a wrong type, while the parser accepts a null anywhere a value may be omitted and treats it as the field being unset. A manifest is checked by running `infra validate` on it, and the modeline only moves part of that feedback earlier.

## Pipeline schedules

A project's pipeline schedules are managed the way its own settings are: `infra import` exports them, `infra validate` checks them, and `infra plan` and `infra apply` create, change and remove them so the project matches what the manifest declares.

A schedule is identified by its description. GitLab addresses one by a server-assigned id that no manifest holds, so the description is what pairs a declared schedule with a live one. GitLab does not require descriptions to be unique, and a project holding two schedules described alike is refused by import, plan and apply, because a change addressed to that description would land on an arbitrary member of the pair. The refusal names both ids, and it costs the project its schedules rather than the project: its other settings are still planned and applied.

Because the description is the identity, editing one is not a rename. The schedule with the old description is removed and a schedule with the new one is created, so it loses its id and everything GitLab keeps against that id.

The `pipeline_schedules` key follows the same rule as `topics`. A manifest that omits the key says nothing about schedules and leaves every live one alone. A manifest that carries the key with an empty list declares that the project holds no schedule, so applying it removes every one that exists. This matters for an exported manifest: `infra import` writes `pipeline_schedules: []` for a project owning none, so committing that document is a declaration that none may be added.

An omitted attribute takes the value GitLab defaults on create, which for `active` is `true`. A declared schedule that does not say `active: false` is therefore a declaration that it runs, and applying such a manifest resumes a schedule someone paused in the web UI.

A schedule's variables are outside the manifest. They are not exported, and a manifest declaring a `variables` block on a schedule is rejected as carrying an unknown field rather than being read and ignored, so a schedule in a manifest describes when a pipeline runs and not what it receives. GitLab requires a value to create a variable, and a manifest that is committed to version control is the wrong place to keep one, so a declared variable could never be created from the document that declares it.

Removing a schedule destroys its variables, and nothing else in a plan would mention them, so a planned removal lists the keys of the variables going with it. Only their keys are read, never their values. GitLab answers a variables read with the field left out for a reader who is neither Maintainer, Owner, nor the schedule's creator; that is reported as a warning and the removal is still shown, because performing it does not depend on the answer.

Writing a schedule needs more than reading one. Reading a project's schedules needs only the access that reads the project, while every write is refused unless the token both holds the Maintainer or Owner role and belongs to the user who created that schedule. A plan can therefore be correct and its apply refused, which the apply error says explicitly rather than blaming the role alone.

## Protected branches

A project's protected branches are managed the way its pipeline schedules are: `infra import` exports them, `infra validate` checks them, and `infra plan` and `infra apply` create, change and remove them so the project matches what the manifest declares.

A protection is identified by its name, which is also how GitLab addresses one. Unlike a schedule, no server-assigned id stands between the manifest and the API, and GitLab refuses a second protection of a name a project already holds, so the pairing rests on a rule GitLab enforces rather than one this tool has to defend. The name may be a wildcard pattern such as `release/*`, and it need not match a branch that exists; GitLab accepts both, and nothing here checks the name against the repository.

The `protected_branches` key follows the same rule as `topics`. A manifest that omits the key says nothing about the protections and leaves every live one alone. A manifest that carries the key with an empty list declares that the project holds none, so applying it unprotects every branch. This matters more here than elsewhere, because GitLab protects a project's default branch when it creates the project: `infra import` writes a `protected_branches` entry for nearly every project, and committing a document with that key emptied is a declaration that no branch may be protected.

A role names a floor rather than an exact role. `developer` is GitLab's access level 30, which it displays as "Developers + Maintainers", so declaring it admits maintainers too. The four values are `no_one`, `developer`, `maintainer` and `admin`, the last being an instance administrator and therefore only meaningful on a self-managed instance. An omitted role takes what GitLab grants on create, which is `maintainer`, and an omitted `allow_force_push` is false.

Who may lift a protection is a key only where GitLab reports one. A Community Edition build accepts `unprotect_access_level` on create and then answers every read without it, so on such an instance there is nothing to declare and an export writes no key; an Enterprise build reports it whether or not it is licensed, so an export from one carries `unprotect_access_level` and a plan compares it. Its values are `developer`, `maintainer` and `admin`: GitLab refuses an unprotect level of "no one", and a protection nobody could lift would be permanent.

Reporting the level is not the same as enforcing it.
GitLab documents the restriction on who may unprotect a branch as a Premium feature, and an Enterprise build without a licence stores the level and reports it back while leaving the decision where a Community Edition build leaves it, with the Maintainer role.
On GitLab EE 19.5.3 with no licence, a maintainer unprotected a branch whose level was `admin`, and a developer could not unprotect one whose level was `developer`.
The key is reconciled on such an instance all the same, because the stored value is what a licensed one enforces and a recreate that dropped it would lose it.

Because the key exists on one kind of instance and not the other, silence about it means "leave it as it is" rather than "clear it". A document exported from a Community Edition instance therefore plans no drift when it is pointed at an Enterprise one, and an update carries the live value through the recreate rather than letting GitLab's create default replace it.

A declaration pointed the other way, at an instance that reports nothing, is left alone rather than applied. Such a value could never be confirmed: applying it would recreate the branch, the next read would report nothing again, and every run would repeat the same change while reopening the branch for the length of it. Whether an instance answers with the field is not knowable from the document, so `infra validate` cannot catch this; `infra plan` says on stderr which branches carry a level it is ignoring. The reasoning is recorded in [ADR-006](../adr/adr-006-the-unprotect-access-level-is-reconciled-only-where-gitlab-reports-it.md).

`code_owner_approval_required` is not a key at all. An Enterprise build reports the field and then refuses to set it without a licence, so it can be neither declared nor reconciled.

A protection GitLab grants to anything the manifest cannot name is refused rather than read as the role beside it. That covers a grant to a user, a group, a custom role or a deploy key, on any of the three access lists, and a protection inherited from a group. A deploy key with push access is the one such shape a project on the free tier can reach, and a document written from a flattened read would revoke that grant the moment it was applied. The refusal costs the project its protections rather than the project: its other settings are still planned and applied.

Changing a role removes the protection and creates it again, so the branch is unprotected between the two calls, and the protection's server-assigned id changes along with anything GitLab keeps against it. GitLab's PATCH accepts a changed access level, answers success and changes nothing, which is why it is not used; the reasoning is recorded in [ADR-005](../adr/adr-005-a-protected-branch-is-reconciled-by-recreating-it.md). An apply whose second call fails reports that the branch was left unprotected, because nothing else in the report would mention it.

## Inspired by

This command is inspired by [babarot/gh-infra](https://github.com/babarot/gh-infra).
