#!/usr/bin/env bash
set -xe

#CGO_ENABLED=0
# Stamp the semantic version (from the committed VERSION file) into the binary
# so `shuttle-go --version` reports it. The Nix build reads the same VERSION
# file, so the two stay in sync.
VERSION="$(head -n1 VERSION)"
GOOS=linux GOARCH=amd64 go build -v -ldflags "-X main.version=${VERSION}" -o shuttle-go
# Emit the desktop launcher next to the binary so the Nix build (which
# packages the built tree) picks it up.
mkdir -p dist
cp shuttle-go.desktop dist/
