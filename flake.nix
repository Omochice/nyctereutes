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
            settings.global.excludes = [ ];
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
            vendorHash = "sha256-QFH+ugrVqXFzov6Z+gQg2rh67+HbNavhG5xSfeCH0Nk=";
            ldflags = [
              "-X github.com/Omochice/nyctereutes/nyctereutes.version=${version}"
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
