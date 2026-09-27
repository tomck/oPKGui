#!/bin/bash
# Builds oPKGui through SynoCommunity's actual `spksrc` framework (via their
# published Docker build image), instead of hand-assembling the .spk format.
# See DESIGN.md for why this replaced the original hand-rolled approach.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPKSRC_DIR="$ROOT/.spksrc"
PKG_SRC="$ROOT/pkgsrc/opkgui"
DIST_DIR="$ROOT/dist"
TCVERSION="${TCVERSION:-7.1}"
ARCH=noarch  # see Makefile's override ARCH=noarch comment for why

echo "Cross-compiling opkgui binary for linux/amd64..."
mkdir -p "$PKG_SRC/src/bin"
( cd "$PKG_SRC/cmd/opkgui" && \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$PKG_SRC/src/bin/opkgui-$ARCH" . )

if [ ! -d "$SPKSRC_DIR" ]; then
    echo "Cloning spksrc (shallow)..."
    git clone --depth 1 https://github.com/SynoCommunity/spksrc.git "$SPKSRC_DIR"
fi

echo "Syncing package source into spksrc tree..."
rm -rf "$SPKSRC_DIR/spk/opkgui"
cp -r "$PKG_SRC" "$SPKSRC_DIR/spk/opkgui"

echo "Building via spksrc Docker image (ARCH=$ARCH, TCVERSION=$TCVERSION)..."
docker run --rm --platform=linux/amd64 \
    -v "$SPKSRC_DIR:/spksrc" -w /spksrc \
    ghcr.io/synocommunity/spksrc \
    make -C spk/opkgui "ARCH=$ARCH" "TCVERSION=$TCVERSION"

mkdir -p "$DIST_DIR"
rm -f "$DIST_DIR"/opkgui_*.spk
cp "$SPKSRC_DIR"/packages/opkgui_*.spk "$DIST_DIR/"
echo "Built:"
ls -la "$DIST_DIR"/opkgui_*.spk
