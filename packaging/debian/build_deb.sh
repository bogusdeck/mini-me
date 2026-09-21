#!/usr/bin/env bash
set -euo pipefail

VERSION="0.1.0"
ARCH="$(dpkg --print-architecture 2>/dev/null || echo "amd64")"
PKG_DIR="dist/mini-me_${VERSION}_${ARCH}"

echo "Building mini-me binary..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/mini-me ./cmd/mini-me

echo "Creating Debian package structure..."
mkdir -p "${PKG_DIR}/usr/local/bin"
mkdir -p "${PKG_DIR}/DEBIAN"

cp dist/mini-me "${PKG_DIR}/usr/local/bin/mini-me"
chmod 755 "${PKG_DIR}/usr/local/bin/mini-me"

cat <<EOF > "${PKG_DIR}/DEBIAN/control"
Package: mini-me
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${ARCH}
Maintainer: bogusdeck <bogusdeck@users.noreply.github.com>
Description: Local-first personal digital twin and RAG context engine
 mini-me scans browser history, local documents, and project repos
 to build a secure, searchable personal knowledge graph stored in SQLite.
EOF

dpkg-deb --build "${PKG_DIR}"
echo "Debian package created successfully: dist/mini-me_${VERSION}_${ARCH}.deb"
