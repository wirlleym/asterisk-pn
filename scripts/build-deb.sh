#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-1.0.0}"
ARCH="${2:-amd64}"
PKG_NAME="asterisk-pn"
PKG_FILE="${PKG_NAME}_${VERSION}_${ARCH}.deb"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

echo "[build] compilando o binario (go build)..."
( cd "$ROOT" && CGO_ENABLED=0 go build -o "$STAGE/asterisk-pn" . )

echo "[build] montando a arvore do pacote..."
mkdir -p "$STAGE/pkg/DEBIAN"
mkdir -p "$STAGE/pkg/usr/local/bin"
mkdir -p "$STAGE/pkg/etc/asterisk-pn"
mkdir -p "$STAGE/pkg/usr/share/doc/$PKG_NAME"

install -m 0755 "$STAGE/asterisk-pn" "$STAGE/pkg/usr/local/bin/asterisk-pn"
install -m 0644 "$ROOT/push.env.example" "$STAGE/pkg/etc/asterisk-pn/push.env.example"
install -m 0644 "$ROOT/PACKAGE-README.md" "$STAGE/pkg/usr/share/doc/$PKG_NAME/README.md"

sed -e "s/^Version:.*/Version: ${VERSION}/" \
    -e "s/^Architecture:.*/Architecture: ${ARCH}/" \
    "$ROOT/debian/control" > "$STAGE/pkg/DEBIAN/control"
install -m 0755 "$ROOT/debian/postinst" "$STAGE/pkg/DEBIAN/postinst"
install -m 0755 "$ROOT/debian/postrm" "$STAGE/pkg/DEBIAN/postrm"

echo "[build] gerando o pacote..."
dpkg-deb --build --root-owner-group "$STAGE/pkg" "$ROOT/$PKG_FILE"

echo "[build] ok: $ROOT/$PKG_FILE"
