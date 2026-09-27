#!/bin/bash
# Hand-rolled SPK builder for oPKGui v0.
#
# Deliberately doesn't depend on cloning/building SynoCommunity's full
# `spksrc` cross-compilation framework -- this package is pure noarch PHP
# and static assets, so a plain tar/gzip of the SPK's known internal layout
# is sufficient. See DESIGN.md for why.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPK_SRC="$ROOT/spk"
BUILD_DIR="$ROOT/build"
INFO_FILE="$SPK_SRC/INFO"

PKG_NAME=$(grep '^package=' "$INFO_FILE" | cut -d'"' -f2)
PKG_VERSION=$(grep '^version=' "$INFO_FILE" | cut -d'"' -f2)
OUT_FILE="$ROOT/${PKG_NAME}-${PKG_VERSION}.spk"

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

chmod +x "$SPK_SRC"/scripts/*

# package.tgz: payload extracted to /var/packages/<pkg>/target/ on install.
tar -czf "$BUILD_DIR/package.tgz" -C "$SPK_SRC/package" .

# Outer SPK container is a plain (uncompressed) tar of INFO + icons + conf +
# scripts + the package.tgz payload, named with a .spk extension.
tar -cf "$OUT_FILE" \
    -C "$SPK_SRC" INFO PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG conf scripts \
    -C "$BUILD_DIR" package.tgz

rm -rf "$BUILD_DIR"

echo "Built $OUT_FILE"
