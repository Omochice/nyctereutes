# nyctereutes

A CLI toolchain for operating GitLab projects, driven through the [glab](https://gitlab.com/gitlab-org/cli) CLI.

## Requirements

The commands talk to GitLab via `glab`, so an authenticated `glab` installation is required.

## Installation

The package is provided as a Nix flake.

```sh
nix profile install github:Omochice/nyctereutes
```

It can also be installed with the Go toolchain.

```sh
go install github.com/Omochice/nyctereutes@latest
```

## Commands

The main commands are documented in their own pages.

- [dep](doc/dep.md) manages dependency-update merge requests.
- [infra](doc/infra.md) manages project settings through YAML manifests.
- [doc](doc/doc.md) reads these pages back out of the installed binary.

The pages are compiled into the binary, so they can be read without leaving the terminal.

```sh
nyctereutes doc list
nyctereutes doc show dep
```

## Development

The development environment is provided by the flake.

```sh
nix develop
go test ./...
nix flake check
```

## License

This project is licensed under the [zlib License](LICENSE).
