# Omada DuckDNS Updater - Agent Documentation

This file serves as a context and reference guide for AI assistants and developers interacting with this project.

## Overview
`omada-duckdns-updater` is a standalone Go daemon that dynamically retrieves WAN IP addresses (IPv4 and IPv6) from a TP-Link Omada SDN Controller and updates DuckDNS. It features an embedded Web UI for interactive configuration and a built-in status dashboard.

## Project Structure
- **`main.go`**: The entry point of the application. It launches the web server in a goroutine and executes a continuous background loop that triggers updates based on the configured `UpdateInterval`.
- **`config.go`**: Manages the `Config` struct. Handles reading from and writing to the plaintext `updater.conf` file. It stores Omada API credentials, DuckDNS details, update intervals, and Web UI Basic Auth credentials.
- **`updater.go`**: Contains the core business logic and API integrations:
  - Connects to the Omada OpenAPI to fetch access tokens (`getOmadaToken`).
  - Retrieves a list of available sites (`fetchSites`).
  - Finds the network gateway and extracts WAN IPv4/IPv6 addresses (`getGatewayWAN`). Note: Omada API uses `siteId` for the site identifier.
  - Submits the IPs to DuckDNS (`updateDuckDNS`).
  - Maintains `globalState` (a thread-safe `UpdateState` struct) to track the timestamp, IPs, and success/error status of the most recent run for the Web UI dashboard.
- **`web.go`**: Implements the HTTP server running on port `5381`.
  - Renders a modern, glass-styled HTML dashboard and configuration form using `html/template`.
  - Endpoints: 
    - `/` : Renders the UI.
    - `/save` : Receives form data to update the configuration.
    - `/run` : Forces an immediate update cycle.
    - `/api/sites` : Proxies Omada API requests to populate the Site ID dropdown.
  - Implements a Basic Authentication middleware that locks down all endpoints if `WebUsername` and `WebPassword` are configured.
- **`updater.conf`**: The auto-generated configuration file. Best managed via the Web UI to ensure valid formatting and immediate application of settings.

## Systemd Integration
The application runs as a user-level systemd service, eliminating the need for cron jobs or timers.
- **Service Name**: `omada-duckdns-updater.service`
- **Location**: `~/.config/systemd/user/omada-duckdns-updater.service`
- **Commands**:
  - Check status: `systemctl --user status omada-duckdns-updater.service`
  - Restart service: `systemctl --user restart omada-duckdns-updater.service`
  - View logs: `journalctl --user -u omada-duckdns-updater.service -f`

## Developer & Agent Guidelines
- **Thread Safety**: The `globalState` variable in `updater.go` is protected by a `sync.RWMutex`. Always use `.RLock()` for reads (especially in `web.go` before passing state to templates) and `.Lock()` for writes.
- **Static Assets**: All CSS, JS, and HTML are embedded directly in `web.go` via a Go template string to keep deployment to a single binary simple. 
- **Dependencies**: The project relies purely on the Go standard library (`net/http`, `encoding/json`, `html/template`, etc.). No third-party modules are required.
