#!/bin/bash
set -e

GIT_TAG=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
DEB_VERSION=${GIT_TAG#v} # Strip leading 'v' for the Debian package version
ARCH=${1:-$(dpkg --print-architecture)}
GOARCH=$ARCH
if [ "$ARCH" = "armhf" ]; then
    GOARCH="arm"
fi
PKG_NAME="omada-duckdns-updater"
BUILD_DIR="build_deb/${PKG_NAME}_${DEB_VERSION}_${ARCH}"

echo "Cleaning up old build directory..."
rm -rf build_deb

echo "Creating directory structure..."
mkdir -p "${BUILD_DIR}/DEBIAN"
mkdir -p "${BUILD_DIR}/opt/${PKG_NAME}"
mkdir -p "${BUILD_DIR}/etc/systemd/system"

echo "Compiling Go binary for ${ARCH}..."
GOOS=linux GOARCH=${GOARCH} go build -ldflags "-X main.version=${GIT_TAG}" -o "${BUILD_DIR}/opt/${PKG_NAME}/${PKG_NAME}"

echo "Creating DEBIAN/control..."
cat <<EOF > "${BUILD_DIR}/DEBIAN/control"
Package: ${PKG_NAME}
Version: ${DEB_VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Maintainer: You <you@example.com>
Description: Omada DuckDNS Updater
 A zero-dependency tool to dynamically update DuckDNS using Omada Controller's WAN IP.
EOF

echo "Creating DEBIAN/postinst..."
cat <<'EOF' > "${BUILD_DIR}/DEBIAN/postinst"
#!/bin/sh
set -e
systemctl daemon-reload
systemctl enable omada-duckdns-updater.service
systemctl start omada-duckdns-updater.service

echo ""
echo "========================================================================"
echo " Omada DuckDNS Updater successfully installed and started in background "
echo "========================================================================"
echo " To complete setup, open your browser and navigate to:"
echo " http://<your-server-ip>:5381/"
echo ""
echo " Note: Configure it immediately to start syncing your IP to DuckDNS."
echo "========================================================================"
echo ""
EOF
chmod 755 "${BUILD_DIR}/DEBIAN/postinst"

echo "Creating DEBIAN/prerm..."
cat <<'EOF' > "${BUILD_DIR}/DEBIAN/prerm"
#!/bin/sh
set -e
if [ "$1" = "remove" ]; then
    systemctl stop omada-duckdns-updater.service || true
    systemctl disable omada-duckdns-updater.service || true
fi
EOF
chmod 755 "${BUILD_DIR}/DEBIAN/prerm"

echo "Creating DEBIAN/postrm..."
cat <<'EOF' > "${BUILD_DIR}/DEBIAN/postrm"
#!/bin/sh
set -e
if [ "$1" = "purge" ]; then
    rm -rf /opt/omada-duckdns-updater
fi
systemctl daemon-reload
EOF
chmod 755 "${BUILD_DIR}/DEBIAN/postrm"

echo "Creating systemd service file..."
cat <<EOF > "${BUILD_DIR}/etc/systemd/system/${PKG_NAME}.service"
[Unit]
Description=Omada DuckDNS Updater Service
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/${PKG_NAME}
ExecStart=/opt/${PKG_NAME}/${PKG_NAME}
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

echo "Building .deb package..."
dpkg-deb --root-owner-group --build "${BUILD_DIR}"
mv "${BUILD_DIR}.deb" .
echo "Build complete: ${PKG_NAME}_${DEB_VERSION}_${ARCH}.deb"
