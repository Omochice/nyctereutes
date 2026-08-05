---
description: Read the documentation compiled into the nyctereutes binary. Read this to learn how document names are formed, or before answering questions about where the documentation for a command lives.
---

# doc

`doc` serves the documentation compiled into the binary, so the pages a build reports are the ones written for that build rather than whatever a web page happens to say.

## Subcommands

The command is split between finding a document and reading it.

- `doc list` reports every embedded document as JSON, giving the name and description of each.
- `doc show <name>` writes one document to standard output unchanged.

## Names

A document is named by its file name with the `.md` extension removed, so `doc/dep.md` is named `dep`.
The documentation is kept in one flat directory, so a name never carries a path the reader would have to learn before typing it.
Passing a name no document carries fails with the names that do exist, so a mistyped name can be corrected without listing again.

## Descriptions

Each document declares its description in YAML frontmatter rather than having one derived from its prose.
The prose says what a command does, while the description says when the document is worth opening, which is what a reader choosing between documents needs.
