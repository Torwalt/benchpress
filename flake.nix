{
  description = "benchpress — a workbench of topics as Markdown files, with a board over them";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    claude-code-nix.url = "github:sadjow/claude-code-nix";
    ploopy = {
      url = "github:Torwalt/ploopy";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.claude-code-nix.follows = "claude-code-nix";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      claude-code-nix,
      ploopy,
    }:
    let
      version = "0.1.0";
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: rec {
        default = benchpress;
        benchpress = pkgs.buildGoModule {
          pname = "benchpress";
          inherit version;
          src = self;
          vendorHash = "sha256-Xbl5vtNPXJ/PWKCrfPZZCQ2yAuZ6lTnFPD2F55/4aw4=";
          subPackages = [ "cmd/benchpress" ];
          nativeCheckInputs = [ pkgs.git ];
          preCheck = "export HOME=$TMPDIR";
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          meta = {
            description = "A workbench of topics as Markdown files, with a board over them";
            mainProgram = "benchpress";
            platforms = nixpkgs.lib.platforms.unix;
          };
        };
      });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = nixpkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.benchpress;
        };
      });

      overlays.default = final: _prev: {
        benchpress = self.packages.${final.stdenv.hostPlatform.system}.benchpress;
      };

      # The shell can run ploopy end to end: all harnesses are here.
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gofumpt
            golangci-lint
            delve
            git
            ripgrep
            jq
            opencode
            codex
            claude-code-nix.packages.${pkgs.stdenv.hostPlatform.system}.claude-code
            ploopy.packages.${pkgs.stdenv.hostPlatform.system}.ploopy
            nixfmt
          ];
        };
      });

      checks = forAllSystems (pkgs: {
        benchpress = self.packages.${pkgs.stdenv.hostPlatform.system}.benchpress;
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
