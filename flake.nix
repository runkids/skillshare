{
  description = "Share skills across AI CLI tools";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        skillshare = pkgs.callPackage ./nix/package.nix { };
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.skillshare;
      });

      overlays.default = final: _prev: {
        skillshare = final.callPackage ./nix/package.nix { };
      };

      checks = forAllSystems (pkgs: {
        skillshare = self.packages.${pkgs.stdenv.hostPlatform.system}.skillshare.overrideAttrs {
          doCheck = true;
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
