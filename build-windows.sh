#!/bin/bash
set -euo pipefail

# Cross-compile Windows amd64 release zip from Linux/macOS CI or a dev machine.
# Usage: ./build-windows.sh [amd64|arm64]

GOARCH=${1:-amd64}
PKG_NAME="omada-duckdns-updater"
GIT_TAG=$(git describe --tags --always 2>/dev/null || echo "dev")
VERSION=${GIT_TAG#v}
OUT_DIR="dist/windows_${GOARCH}"
ZIP_NAME="${PKG_NAME}_${VERSION}_windows_${GOARCH}.zip"

echo "Building ${PKG_NAME} for windows/${GOARCH} (version ${VERSION})..."
rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

CGO_ENABLED=0 GOOS=windows GOARCH="${GOARCH}" go build \
  -ldflags "-X main.version=${GIT_TAG}" \
  -o "${OUT_DIR}/${PKG_NAME}.exe"

cp scripts/install-windows.ps1 "${OUT_DIR}/"
cp scripts/uninstall-windows.ps1 "${OUT_DIR}/"
cp updater.conf.example "${OUT_DIR}/"
cp README.md "${OUT_DIR}/"

rm -f "${ZIP_NAME}"
(
  cd "${OUT_DIR}"
  zip -r "../../${ZIP_NAME}" .
)

echo "Created ${ZIP_NAME}"
ls -lh "${ZIP_NAME}"
