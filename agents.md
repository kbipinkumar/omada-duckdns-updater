# Omada DuckDNS Updater - Agent Documentation

This file serves as a context and reference guide for AI assistants and developers interacting with this project.

## Overview
`omada-duckdns-updater` is a standalone Go daemon that dynamically retrieves WAN IP addresses (IPv4 and IPv6) from a TP-Link Omada SDN Controller and updates DuckDNS. It features an embedded Web UI for interactive configuration and a built-in status dashboard.

## Project Structure
- **`main.go`**: Entry point. Parses flags, optionally handles Windows `-service` commands, then runs `runApp` (web server + context-cancelled updater loop). Console mode uses SIGINT/SIGTERM.
- **`config.go`**: Manages the `Config` struct and `updater.conf`. Uses `filepath.Join` and `DATA_DIR`. On Windows, defaults to `%ProgramData%\omada-duckdns-updater` when `DATA_DIR` is unset; on other OSes defaults to `./updater.conf` (cwd).
- **`updater.go`**: Contains the core business logic and API integrations:
  - Connects to the Omada OpenAPI to fetch access tokens (`getOmadaToken`).
  - Retrieves a list of available sites (`fetchSites`).
  - Finds the network gateway and extracts WAN IPv4/IPv6 addresses (`getGatewayWAN`). Note: Omada API uses `siteId` for the site identifier.
  - Submits the IPs to DuckDNS (`updateDuckDNS`).
  - Maintains `globalState` (a thread-safe `UpdateState` struct) to track the timestamp, IPs, and success/error status of the most recent run for the Web UI dashboard.
- **`web.go`**: HTTP server on port `5381` with graceful `Shutdown` on context cancel.
  - Endpoints: `/`, `/save`, `/run`, `/api/sites`
  - Basic Auth when `WebUsername` / `WebPassword` are set.
- **`service_windows.go`** / **`service_other.go`**: Windows Service integration (`OmadaDuckDNSUpdater`) via `golang.org/x/sys/windows/svc`; stubs on non-Windows.
- **`scripts/install-windows.ps1`** / **`uninstall-windows.ps1`**: Admin installers for Windows Desktop/Server.
- **`build-windows.sh`**: Cross-compiles `windows/amd64` zip for releases.
- **`updater.conf`**: Auto-generated configuration file. Prefer the Web UI for edits.

## Systemd Integration (Linux)
The application can run as a user-level or system systemd service.
- **Service Name**: `omada-duckdns-updater.service`
- **User unit location**: `~/.config/systemd/user/omada-duckdns-updater.service`
- **Commands**:
  - Check status: `systemctl --user status omada-duckdns-updater.service`
  - Restart service: `systemctl --user restart omada-duckdns-updater.service`
  - View logs: `journalctl --user -u omada-duckdns-updater.service -f`

## Windows Service Integration
- **Service Name**: `OmadaDuckDNSUpdater`
- **Display Name**: Omada DuckDNS Updater
- **Install dir**: `C:\Program Files\omada-duckdns-updater\`
- **Data dir**: `%ProgramData%\omada-duckdns-updater\` (`updater.conf`, `updater.log`)
- **Flags**: `-service install|uninstall|start|stop`
- **Logs**: `%ProgramData%\omada-duckdns-updater\updater.log` and Windows Event Log source `OmadaDuckDNSUpdater`
- **Commands**: `Get-Service OmadaDuckDNSUpdater`, `Restart-Service OmadaDuckDNSUpdater`

## Developer & Agent Guidelines
- **Thread Safety**: The `globalState` variable in `updater.go` is protected by a `sync.RWMutex`. Always use `.RLock()` for reads (especially in `web.go` before passing state to templates) and `.Lock()` for writes.
- **Static Assets**: All CSS, JS, and HTML are embedded directly in `web.go` via a Go template string to keep deployment to a single binary simple.
- **Dependencies**: Runtime logic uses the Go standard library plus `golang.org/x/sys` for Windows Service / Event Log support (build-tagged; Linux builds still work with the same module).
- **Cross-compile**: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` or `./build-windows.sh amd64`.
