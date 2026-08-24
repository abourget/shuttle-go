{
  lib,
  buildGoModule,
  fetchFromGitHub,
  dbus,
  libinput,
  xinput,
  procps,
  bash,
  makeWrapper,
}:

let
  runtimeDeps = [
    dbus
    libinput
    xinput
    procps
    bash
  ];
in
buildGoModule rec {
  pname = "shuttle-go";
  version = "1.2.3";

  src = ./.;

  subPackages = [];

  nativeBuildInputs = [ makeWrapper ] ++ runtimeDeps;
  vendorHash = "sha256-GwKZyo6yD1tTsuf0PKc6Ys77FtaXza/Ub8sStnUUl3E=";
  doCheck = false;

  # Stamp the semantic version into the binary so `shuttle-go --version`
  # reports it. The Nix source has no .git, so we read the committed VERSION
  # file (the single source of truth, also used by build.sh).
  ldflags = [ "-X main.version=${lib.removeSuffix "\n" (builtins.readFile ./VERSION)}" ];

  postInstall = ''
    wrapProgram $out/bin/shuttle-go \
      --prefix PATH ":" ${lib.makeBinPath runtimeDeps}

    # Desktop launcher, so the app shows up in the GNOME/KDE application menu.
    # build.sh copies the file into dist/; fall back to the source tree if the
    # copy is absent (e.g. the tree was not built with build.sh first).
    mkdir -p $out/share/applications
    if [ -f $src/dist/shuttle-go.desktop ]; then
      cp $src/dist/shuttle-go.desktop $out/share/applications/
    else
      cat > $out/share/applications/shuttle-go.desktop <<'DESKTOP'
[Desktop Entry]
Name=Shuttle Go
Comment=Contour Design Shuttle Pro V2 driver
Exec=shuttle-go
Icon=shuttle-go
Type=Application
Terminal=false
Categories=Utility;
DESKTOP
    fi

    # Hicolor icon set (downscaled from the Shuttle Pro V2 product photo), so
    # the launcher shows an icon in the application menu. The PNGs are
    # committed under icons/ in the source tree.
    if [ -f $src/icons/hicolor/128x128/apps/shuttle-go.png ]; then
      mkdir -p $out/share/icons
      cp -r $src/icons/hicolor $out/share/icons/
    fi
  '';

  meta = with lib; {
    description = "Contour Design Shuttle Pro V2 drivers for Linux with modifiers";
    homepage = "https://github.com/abourget/shuttle-go";
    license = licenses.mit;
    maintainers = [];
    platforms = platforms.linux;
  };
}
