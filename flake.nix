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
        gitHooks = git-hooks.lib.${system}.run {
          src = self;
          hooks = {
            # keep-sorted start block=yes
            actionlint.enable = true;
            ghalint = {
              enable = true;
              name = "ghalint";
              entry = "${pkgs.ghalint}/bin/ghalint run";
              files = "^\\.github/workflows/.*$";
              pass_filenames = false;
            };
            renovate-config-validator = {
              enable = true;
              name = "renovate-config-validator";
              entry = "${pkgs.renovate}/bin/renovate-config-validator --strict";
              files = "^renovate\\.json5$";
            };
            treefmt = {
              enable = true;
              packageOverrides.treefmt = treefmt.config.build.wrapper;
            };
            zizmor = {
              enable = true;
              name = "zizmor";
              entry = "${pkgs.zizmor}/bin/zizmor .github/workflows .github/actions";
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
        };
        devShells.default = pkgs.mkShell {
          buildInputs = gitHooks.enabledPackages ++ [
            treefmt.config.build.wrapper
          ];
          inherit (gitHooks) shellHook;
        };
        formatter = treefmt.config.build.wrapper;
        # keep-sorted end
      }
    );
}
