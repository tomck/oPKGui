#!/bin/bash
# Builds oPKGui through SynoCommunity's actual `spksrc` framework (via their
# published Docker build image), instead of hand-assembling the .spk format.
# See DESIGN.md for why this replaced the original hand-rolled approach.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPKSRC_DIR="$ROOT/.spksrc"
PKG_SRC="$ROOT/pkgsrc/opkgui"
DIST_DIR="$ROOT/dist"
TCVERSION="${TCVERSION:-7.2}"

if [ ! -d "$SPKSRC_DIR" ]; then
    echo "Cloning spksrc (shallow)..."
    git clone --depth 1 https://github.com/SynoCommunity/spksrc.git "$SPKSRC_DIR"
fi

echo "Syncing package source into spksrc tree..."
rm -rf "$SPKSRC_DIR/spk/opkgui"
cp -r "$PKG_SRC" "$SPKSRC_DIR/spk/opkgui"

echo "Building via spksrc Docker image (ARCH=noarch, TCVERSION=$TCVERSION)..."
docker run --rm --platform=linux/amd64 \
    -v "$SPKSRC_DIR:/spksrc" -w /spksrc \
    ghcr.io/synocommunity/spksrc \
    make -C spk/opkgui ARCH=noarch "TCVERSION=$TCVERSION"

mkdir -p "$DIST_DIR"
cp "$SPKSRC_DIR"/packages/opkgui_*.spk "$DIST_DIR/"
echo "Built:"
ls -la "$DIST_DIR"/opkgui_*.spk
