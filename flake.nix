{
  description = "Contour Design Shuttle Pro V2 drivers for Linux with modifiers";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        shuttleGo = pkgs.callPackage ./shuttle-go.nix { };
        shuttlePro = pkgs.callPackage ./gnome-extension/shuttle-pro.nix { };
      in
      {
        packages.shuttle-go = shuttleGo;
        packages.default = shuttleGo;
        packages.shuttle-pro = shuttlePro;

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            dbus
            libinput
          ];
        };
      });
}
