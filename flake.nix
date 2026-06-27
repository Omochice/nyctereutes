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
              "nlreturn" # blank-line-before-return style, overlaps wsl
              "noinlineerr" # forbids the idiomatic inline error check
              "nonamedreturns" # named returns are used deliberately in tests
              "paralleltest" # t.Parallel() adds little to this small suite
              "tagalign" # struct-tag alignment belongs to formatting
              "testpackage" # white-box tests are intentional here
              "wsl" # opinionated whitespace/cuddling rules
              "wsl_v5" # successor of wsl, same opinionated whitespace rules
              # keep-sorted end
            ];
          };
          formatters.enable = [
            # keep-sorted start
            "gofmt"
            "goimports"
            # keep-sorted end
          ];
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
            # The treefmt-nix `golangci-lint` program runs `golangci-lint run
            # --fix`, which needs dependency type information and fails inside
            # the sealed git-hooks sandbox. Only the `fmt` subcommand is wanted
            # here, so define the formatter directly; `go` is added to PATH for
            # the goimports formatter.
            settings.formatter.golangci-lint = {
              command = pkgs.lib.getExe (
                pkgs.writeShellScriptBin "golangci-lint-fmt" ''
                  export PATH="${pkgs.go}/bin:$PATH"
                  exec ${pkgs.lib.getExe pkgs.golangci-lint} fmt --config ${golangciConfig} "$@"
                ''
              );
              includes = [ "*.go" ];
            };
            programs = {
              # keep-sorted start block=yes
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
        nyctereutes = pkgs.buildGoModule {
          pname = "nyctereutes";
          version = "0.1.0";
          src = self;
          vendorHash = "sha256-W6XVd68MS0ungMgam8jefYMVhyiN6/DB+bliFzs2rdk=";
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
          golangci-lint = golangci-lint-check;
          inherit nyctereutes;
        };
        devShells.default = pkgs.mkShell {
          buildInputs = gitHooks.enabledPackages ++ [
            pkgs.golangci-lint
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
