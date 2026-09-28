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
# Real per-arch builds (ARCH=x64/aarch64/...) need spksrc's own toolchain
# download for that arch even though we don't compile with it -- and that
# step's tar extraction breaks under Docker Desktop's bind mount on macOS
# (see DESIGN.md's Round 4 / TESTING.md, reproduced again with aarch64).
# Real archs build fine in CI on Linux (.github/workflows/build.yml) --
# default stays noarch here so local dev on macOS is unaffected.
ARCH="${ARCH:-noarch}"

case "$ARCH" in
    noarch) GOARCH=amd64; GOARM= ;;
    x64)    GOARCH=amd64; GOARM= ;;
    aarch64) GOARCH=arm64; GOARM= ;;
    armv7)  GOARCH=arm; GOARM=7 ;;
    *) echo "Unknown ARCH=$ARCH (add its GOARCH/GOARM mapping here)" >&2; exit 1 ;;
esac

echo "Cross-compiling opkgui binary for linux/$GOARCH (ARCH=$ARCH)..."
mkdir -p "$PKG_SRC/src/bin"
( cd "$PKG_SRC/cmd/opkgui" && \
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" GOARM="$GOARM" \
  go build -ldflags="-s -w" -o "$PKG_SRC/src/bin/opkgui-$ARCH" . )

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
