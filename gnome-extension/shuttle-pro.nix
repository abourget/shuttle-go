{
  lib,
  stdenvNoCC,
}:

# GNOME Shell extension that exposes the focused window (title, wm_class) and
# the current pointer position over the session D-Bus, so shuttle-go can read
# them on Wayland. Modeled on nixpkgs' buildGnomeExtension, but sourced from
# this repository rather than extensions.gnome.org.
let
  # The extension UUID is the name of the directory the shell looks for. It
  # must stay in sync with the "uuid" field in metadata.json.
  uuid = "shuttle-pro@shuttle-go.dev";
in
stdenvNoCC.mkDerivation {
  pname = "gnome-shell-extension-shuttle-pro";
  version = "1.0";

  src = ./.;

  # There is nothing to compile; the source dir carries a Makefile that
  # stdenv's default build phase would otherwise try to run.
  dontBuild = true;

  # Lay the two files the shell loads where it expects them.
  installPhase = ''
    mkdir -p $out/share/gnome-shell/extensions/${uuid}
    cp extension.js metadata.json $out/share/gnome-shell/extensions/${uuid}/
  '';

  meta = with lib; {
    description = "GNOME Shell extension exposing the focused window and pointer position over D-Bus";
    homepage = "https://github.com/mog/shuttle-go";
    license = licenses.gpl2Plus;
    maintainers = [];
    platforms = platforms.linux;
  };
}
