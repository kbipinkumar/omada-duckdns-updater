# Maintainer: kbipinkumar <kbipinkumar[at]pm.me>

pkgname=omada-duckdns-updater-git
pkgver=1.0.0
pkgrel=1
pkgdesc="A zero-dependency tool to dynamically update DuckDNS using Omada Controller's WAN IP."
arch=('x86_64')
url="https://github.com/kbipinkumar/omada-duckdns-updater"
license=('GPL2')
makedepends=('go' 'git')
source=("omada-duckdns-updater::git+https://github.com/kbipinkumar/omada-duckdns-updater.git")
sha256sums=('SKIP')

pkgver() {
  cd "$srcdir/omada-duckdns-updater"
  # Generate a package version
  git describe --long --tags --always 2>/dev/null | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g' || echo "1.0.0"
}

build() {
  cd "$srcdir/omada-duckdns-updater"
  
  export CGO_ENABLED=0
  GIT_TAG=$(git describe --tags --always 2>/dev/null || echo "v${pkgver}")
  
  # Inject the Git tag into the binary at compile time
  go build -ldflags "-X main.version=${GIT_TAG}" -o "omada-duckdns-updater" .
}

package() {
  cd "$srcdir/omada-duckdns-updater"
  
  # Install binary to /opt since the application writes updater.conf in its current working directory
  install -Dm755 "omada-duckdns-updater" "${pkgdir}/opt/omada-duckdns-updater/omada-duckdns-updater"
  
  # Generate and install systemd service file
  install -d "${pkgdir}/usr/lib/systemd/system"
  cat <<EOF > "${pkgdir}/usr/lib/systemd/system/omada-duckdns-updater.service"
[Unit]
Description=Omada DuckDNS Updater Service
After=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/omada-duckdns-updater
ExecStart=/opt/omada-duckdns-updater/omada-duckdns-updater
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

}
