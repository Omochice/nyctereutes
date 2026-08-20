{
  description = "glab toolchain";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nur-packages = {
      url = "github:Omochice/nur-packages";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    git-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      treefmt-nix,
      flake-utils,
      nur-packages,
      git-hooks,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            nur-packages.overlays.default
          ];
        };
        golangciConfig = (pkgs.formats.yaml { }).generate "golangci.yaml" {
          version = "2";
          linters = {
            default = "all";
            disable = [
              # keep-sorted start
              "depguard" # requires an explicit import policy to be useful
              "exhaustruct" # test fixtures and go-flags structs init only some fields
              "godoclint" # forces godoc comments to restate the symbol name
              "gomodguard" # deprecated in v2.12, superseded by gomodguard_v2
              "nlreturn" # blank-line-before-return style, overlaps wsl
              "noinlineerr" # forbids the idiomatic inline error check
              "nonamedreturns" # named returns are used deliberately in tests
              "paralleltest" # t.Parallel() adds little to this small suite
              "revive" # forces restating doc comments on every export
              "tagalign" # struct-tag alignment belongs to formatting
              "tagliatelle" # JSON tags mirror GitLab's snake_case API
              "testpackage" # white-box tests are intentional here
              "wsl" # opinionated whitespace/cuddling rules
              "wsl_v5" # successor of wsl, same opinionated whitespace rules
              # keep-sorted end
            ];
            # The schema generator looks these methods up on the value type
            # only, so they cannot take the pointer receiver their type's
            # decoding methods need. recvcheck ships the same exclusion for
            # MarshalJSON and MarshalYAML, which are forced the same way.
            settings.recvcheck.exclusions = [
              # keep-sorted start
              "*.JSONSchema"
              "*.JSONSchemaExtend"
              # keep-sorted end
            ];
            exclusions.rules = [
              {
                # Test fixtures state their data as literals on purpose; forcing
                # them into shared constants, wrapping simulated errors, or
                # naming static sentinels hurts test readability.
                path = "_test\\.go";
                linters = [
                  # keep-sorted start
                  "err113"
                  "goconst"
                  "wrapcheck"
                  # keep-sorted end
                ];
              }
            ];
          };
        };
        treefmt = treefmt-nix.lib.evalModule pkgs (
          { ... }:
          let
            rumdlConfig = (pkgs.formats.toml { }).generate "rumdl.toml" {
              # keep-sorted start
              MD004.style = "dash";
              MD007.indent = 4;
              MD007.style = "fixed";
              MD041.enabled = false;
              MD049.style = "underscore";
              MD050.style = "asterisk";
              MD055.style = "leading-and-trailing";
              MD060.enabled = true;
              MD060.style = "aligned";
              MD077.enabled = false;
              global.line_length = 0;
              # keep-sorted end
            };
          in
          {
            settings.global.excludes = [ "CHANGELOG.md" ];
            settings.formatter.rumdl-format.options = [
              "--config"
              (toString rumdlConfig)
            ];
            programs = {
              # keep-sorted start block=yes
              gofumpt.enable = true;
              goimports.enable = true;
              keep-sorted.enable = true;
              nixfmt.enable = true;
              rumdl-format.enable = true;
              toml-sort.enable = true;
              yamlfmt = {
                enable = true;
                settings = {
                  formatter = {
                    type = "basic";
                    retain_line_breaks_single = true;
                  };
                };
              };
              # keep-sorted end
            };
          }
        );
        nyctereutes =
          let
            version = pkgs.lib.pipe ./.github/release-please-manifest.json [
              builtins.readFile
              builtins.fromJSON
              (pkgs.lib.getAttr ".")
            ];
          in
          pkgs.buildGoModule {
            pname = "nyctereutes";
            inherit version;
            src = self;
            vendorHash = "sha256-qQ/VZ2G5IE8t72CGSyrAJ9glhfirNvkUX+KY7oEKs+w=";
            ldflags = [
              "-X github.com/Omochice/nyctereutes/nyctereutes.version=${version}"
              # The manifest version names the last release rather than this
              # tree, so it cannot say where these sources are published. A
              # clean tree is published at its own revision; a dirty one is
              # published nowhere, and the branch it was built off is the
              # closest honest answer.
              "-X github.com/Omochice/nyctereutes/nyctereutes.sourceRef=${self.rev or "refs/heads/main"}"
            ];
          };
        # Run golangci-lint by reusing buildGoModule's module fetching so the
        # dependency type information is available inside the sealed
        # `nix flake check` sandbox, where the git-hooks runner cannot reach
        # the network.
        golangci-lint-check = nyctereutes.overrideAttrs (previousAttrs: {
          pname = "${previousAttrs.pname}-golangci-lint";
          nativeBuildInputs = (previousAttrs.nativeBuildInputs or [ ]) ++ [
            pkgs.golangci-lint
          ];
          doCheck = false;
          buildPhase = ''
            runHook preBuild
            export HOME="$TMPDIR"
            export GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint-cache"
            golangci-lint run --config ${golangciConfig} ./...
            runHook postBuild
          '';
          installPhase = ''
            runHook preInstall
            touch "$out"
            runHook postInstall
          '';
        });
        # godoclint needs the same dependency type information as golangci-lint,
        # so reuse buildGoModule's module fetching for the sealed sandbox.
        # start-with-name and pkg-doc force restating the symbol/package name,
        # and max-len enforces a wrap width the reader's editor should own.
        godoclint-check = nyctereutes.overrideAttrs (previousAttrs: {
          pname = "${previousAttrs.pname}-godoclint";
          nativeBuildInputs = (previousAttrs.nativeBuildInputs or [ ]) ++ [
            pkgs.godoclint
          ];
          doCheck = false;
          buildPhase = ''
            runHook preBuild
            export HOME="$TMPDIR"
            godoclint -default=all -disable=start-with-name,pkg-doc,max-len ./...
            runHook postBuild
          '';
          installPhase = ''
            runHook preInstall
            touch "$out"
            runHook postInstall
          '';
        });
        gopls-check = pkgs.writeShellApplication {
          name = "gopls-check";
          runtimeInputs = [
            pkgs.git
            pkgs.go
            pkgs.gopls
          ];
          text = ''
            # A GOROOT inherited from another Go installation makes gopls load a
            # compiler that disagrees with the `go` on PATH.
            export GOROOT="${pkgs.go}/share/go"
            # gopls takes file names rather than package patterns; listing them
            # through git keeps ignored trees such as scratch worktrees out.
            mapfile -t files < <(git ls-files --cached --others --exclude-standard '*.go')
            if [ ''${#files[@]} -eq 0 ]; then
              exit 0
            fi
            # `gopls check` exits 0 even when it reports diagnostics, so failure
            # has to be derived from the output being non-empty.
            diagnostics="$(gopls check -severity=hint "''${files[@]}" 2>&1)"
            if [ -n "$diagnostics" ]; then
              printf '%s\n' "$diagnostics" >&2
              exit 1
            fi
          '';
        };
        gitHooks = git-hooks.lib.${system}.run {
          src = self;
          hooks = {
            # keep-sorted start block=yes
            actionlint.enable = true;
            ghalint = {
              enable = true;
              name = "ghalint";
              entry = "${pkgs.lib.getExe pkgs.ghalint} run";
              files = "^\\.github/workflows/.*$";
              pass_filenames = false;
            };
            gitleaks = {
              enable = true;
              name = "gitleaks";
              entry = "${pkgs.lib.getExe pkgs.gitleaks} git --pre-commit --redact --staged --verbose --no-banner";
              pass_filenames = false;
            };
            renovate-config-validator = {
              enable = true;
              name = "renovate-config-validator";
              entry = "${pkgs.lib.getExe' pkgs.renovate "renovate-config-validator"} --strict";
              files = "^renovate\\.json5$";
            };
            treefmt = {
              enable = true;
              packageOverrides.treefmt = treefmt.config.build.wrapper;
            };
            zizmor = {
              enable = true;
              name = "zizmor";
              entry = "${pkgs.lib.getExe pkgs.zizmor} .github/workflows .github/actions";
              files = "^\\.github/(workflows|actions)/.*$";
              pass_filenames = false;
            };
            # keep-sorted end
          };
        };
      in
      {
        # keep-sorted start block=yes
        apps.gopls-check = flake-utils.lib.mkApp { drv = gopls-check; };
        checks = {
          git-hooks = gitHooks;
          godoclint = godoclint-check;
          golangci-lint = golangci-lint-check;
          inherit nyctereutes;
        };
        devShells.default = pkgs.mkShell {
          buildInputs = gitHooks.enabledPackages ++ [
            pkgs.go
            pkgs.godoclint
            pkgs.golangci-lint
            pkgs.nix-update
            pkgs.octocov
            treefmt.config.build.wrapper
          ];
          inherit (gitHooks) shellHook;
        };
        formatter = treefmt.config.build.wrapper;
        packages.default = nyctereutes;
        # keep-sorted end
      }
    );
}
