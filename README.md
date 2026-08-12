# Omada DuckDNS Updater

A lightweight, zero-dependency Go application that dynamically fetches your WAN IP addresses (both IPv4 and IPv6) from a TP-Link Omada SDN Controller and automatically updates your DuckDNS domain.

It features a beautiful, built-in Web UI for easy configuration and a live status dashboard, removing the need to mess with command-line arguments or `.env` files.

## Features
- 🚀 **Minimal Dependencies**: Single Go binary (stdlib + `golang.org/x/sys` for Windows services and `golang.org/x/crypto` for password hashing).
- 📊 **Live Dashboard**: View the last run time, success/error status, and fetched IPs directly from the UI.
- 🔒 **Basic Authentication**: Secure your Web UI with optional username and password protection (bcrypt-hashed at rest).
- ⏱️ **Dynamic Scheduling**: Update intervals are fully configurable from the Web UI—no need to restart the application.
- 🌐 **Dual Stack**: Supports updating both IPv4 and IPv6 addresses simultaneously.
- 🔍 **Site Auto-Discovery**: Automatically fetches and lists your available Omada sites to prevent misconfiguration.
- 🪟 **Windows Service**: Native Windows Service install for Desktop and Server (alongside Linux systemd and Docker).

---

## Prerequisites
- **Go 1.26+** (if compiling from source)
- An **Omada SDN Controller** (Hardware or Software) with an API user / Client ID & Secret configured.
- A **DuckDNS** account and domain.

---

## Installation & Deployment

You can run `omada-duckdns-updater` on Linux (systemd or Docker), on Windows (native service or console), or via Docker Desktop on Windows using the existing Linux images.

### Method 1: Debian Package (.deb) (Recommended for Debian/Ubuntu)

1. **Build the package:**
   ```bash
   git clone https://github.com/kbipinkumar/omada-duckdns-updater.git
   cd omada-duckdns-updater
   ./build-deb.sh
   ```
2. **Install the package:**
   ```bash
   sudo dpkg -i omada-duckdns-updater_*.deb
   ```
   *(This automatically installs the binary to `/opt`, creates the systemd service, and starts it in the background).*

### Method 2: Arch Linux Package (Recommended for Arch/Manjaro)

A `PKGBUILD` is provided to seamlessly integrate with Arch Linux.

1. **Build and Install:**
   ```bash
   git clone https://github.com/kbipinkumar/omada-duckdns-updater.git
   cd omada-duckdns-updater
   makepkg -si
   ```
2. **Enable and Start:**
   ```bash
   sudo systemctl enable --now omada-duckdns-updater.service
   ```

### Method 3: Systemd (Manual Compilation)

1. **Clone and Build:**
   ```bash
   go build -o omada-duckdns-updater
   ```

2. **Create a Systemd User Service:**
   Create a file at `~/.config/systemd/user/omada-duckdns-updater.service`:
   ```ini
   [Unit]
   Description=Omada DuckDNS Updater Service
   After=network-online.target

   [Service]
   Type=simple
   WorkingDirectory=/path/to/omada-duckdns-updater
   ExecStart=/path/to/omada-duckdns-updater/omada-duckdns-updater
   Restart=on-failure
   RestartSec=10

   [Install]
   WantedBy=default.target
   ```

3. **Enable and Start:**
   ```bash
   systemctl --user daemon-reload
   systemctl --user enable --now omada-duckdns-updater.service
   ```
   *(**Important Note for Headless Environments**: If you are running this on a headless server without a persistent desktop session, the user systemd manager will shut down when you terminate your SSH session. To keep the service running continuously in the background, you MUST run: `loginctl enable-linger $USER`)*

### Method 4: Docker (GitHub Container Registry)

Pre-built Docker images are automatically published to the GitHub Container Registry for both `amd64` and `arm64` architectures.

1. **Run using Docker Compose (Recommended):**
   A `docker-compose.yml` file is included in the repository.
   ```bash
   docker compose up -d
   ```
   *(This will automatically pull the latest image from GHCR and mount `updater.conf` as a volume to ensure your configuration persists).*

   **Optional: environment secret overrides**

   For GitOps or secret-manager workflows, you can supply API tokens and the Web UI password via environment variables instead of storing them in `updater.conf`:

   | Variable | Effect |
   |----------|--------|
   | `OMADA_CLIENT_SECRET` | Overrides the Omada client secret from `updater.conf` at runtime |
   | `DUCKDNS_TOKEN` | Overrides the DuckDNS token from `updater.conf` at runtime |
   | `WEB_PASSWORD` | Overrides the Web UI password at runtime (plaintext in env only; never written to `updater.conf`) |

   Copy `example.env` to `.env`, set restrictive permissions, replace the placeholder values, then start the stack:

   ```bash
   cp example.env .env
   chmod 600 .env
   # edit .env with your secrets
   docker compose up -d
   ```

   `WEB_USERNAME` must still be set in `updater.conf` (via the Web UI) for Basic Auth to be enabled. Only the password can be overridden via `WEB_PASSWORD`.

2. **Run using Docker CLI:**
   ```bash
   cp example.env .env
   chmod 600 .env
   # edit .env with your secrets (OMADA_CLIENT_SECRET, DUCKDNS_TOKEN, WEB_PASSWORD)
   docker run -d \
     --name omada-ddns \
     -p 5381:5381 \
     -e DATA_DIR=/data \
     --env-file .env \
     -v omada-data:/data \
     ghcr.io/kbipinkumar/omada-duckdns-updater:latest
   ```

### Method 5: Docker (Local Development)

If you are modifying the code and want to build the Docker image locally:

1. **Using Docker Compose:**
   ```bash
   cp example.env .env   # optional: add secret overrides
   chmod 600 .env
   # edit .env with your secrets
   docker compose -f docker-compose.dev.yml up -d --build
   ```

   `docker-compose.dev.yml` supports the same optional `OMADA_CLIENT_SECRET`, `DUCKDNS_TOKEN`, and `WEB_PASSWORD` environment overrides as the production compose file.

2. **Using the Build Wrapper:**
   We also provide a wrapper script that automatically tags your locally built image with the correct Git version.
   ```bash
   ./build-docker.sh
   cp example.env .env
   chmod 600 .env
   # edit .env with your secrets (OMADA_CLIENT_SECRET, DUCKDNS_TOKEN, WEB_PASSWORD)
   docker run -d -p 5381:5381 \
     -e DATA_DIR=/data \
     --env-file .env \
     -v omada-data:/data \
     omada-duckdns-updater:latest
   ```

### Method 6: Windows (Desktop or Server)

Pre-built Windows zip archives are published on GitHub Releases for both architectures:
`omada-duckdns-updater_*_windows_amd64.zip` (x64) and `omada-duckdns-updater_*_windows_arm64.zip` (Windows on Arm). Download the zip that matches your machine.

1. **Download and extract** the Windows zip from the [Releases](https://github.com/kbipinkumar/omada-duckdns-updater/releases) page.
2. **Install as a Windows Service** (Administrator PowerShell):
   ```powershell
   Set-ExecutionPolicy -Scope Process Bypass
   .\install-windows.ps1
   ```
   This copies the binary to `C:\Program Files\omada-duckdns-updater\`, creates `%ProgramData%\omada-duckdns-updater\` for config/logs, opens inbound TCP **5381** on Domain/Private (Public only if confirmed or `-AllowPublicFirewall`), and installs/starts the `OmadaDuckDNSUpdater` service.
3. **Uninstall:**
   ```powershell
   .\uninstall-windows.ps1
   # Keep config/logs:
   .\uninstall-windows.ps1 -KeepData
   ```
4. **Manual / console run** (development or testing, no service):
   ```powershell
   .\omada-duckdns-updater.exe
   ```
5. **Service management:**
   ```powershell
   Get-Service OmadaDuckDNSUpdater
   Restart-Service OmadaDuckDNSUpdater
   # Or via the binary:
   & "C:\Program Files\omada-duckdns-updater\omada-duckdns-updater.exe" -service stop
   & "C:\Program Files\omada-duckdns-updater\omada-duckdns-updater.exe" -service start
   ```
6. **Paths:**
   - Config: `%ProgramData%\omada-duckdns-updater\updater.conf`
   - Log file: `%ProgramData%\omada-duckdns-updater\updater.log`
   - Event Log source: `OmadaDuckDNSUpdater`
7. **Firewall:** the installer adds an inbound allow rule for TCP 5381 on the **Domain** and **Private** profiles. Include Public only via the interactive prompt or `-AllowPublicFirewall`. For a custom setup, allow that port so the Web UI is reachable.
8. **Docker Desktop on Windows:** use the existing Linux GHCR image (Linux containers / WSL2) with the same `docker compose` / `docker run` commands as on Linux. There is no separate Windows container image.

To cross-compile a Windows zip from a Linux/macOS machine:
```bash
./build-windows.sh amd64
./build-windows.sh arm64
```

---

## Usage & Configuration

Once the service is running, open your web browser and navigate to:
**`http://<your-server-ip>:5381/`**

1. **Omada Configuration:** Enter your Omada Controller URL, Client ID, Client Secret, and Omadac ID. 
2. **Fetch Sites:** Click the "Fetch" button next to the Site ID field. The application will securely query your Omada controller and populate a dropdown list of your available sites.
3. **DuckDNS Configuration:** Enter your DuckDNS Token and Domain (e.g., `myhome.duckdns.org`).
4. **System Settings:** 
   - Set your preferred **Update Interval** (default is 5 minutes).
   - Enter a **Web UI Username** and **Password** to secure the dashboard. (Once saved, the page will instantly require these credentials).
5. **First-time setup:** On the first visit (before configuration is saved), click **Verify Connection** to fetch WAN IPv4/IPv6 from your Omada controller and review them in the preview panel. When the addresses look correct, click **Confirm & Save Configuration**.
6. **Run:** Click **Run Now** to force an immediate DuckDNS update (or wait for the scheduled interval).

Check the **Status Dashboard** at the top of the page to verify that the IPs were fetched and DuckDNS was updated successfully!

---

## Security Notes

- **Web UI passwords** are stored as bcrypt hashes in `updater.conf`. If you upgraded from an older release that used salted SHA-256, clear `WEB_PASSWORD` in `updater.conf` and set a new password via the Web UI (see Troubleshooting).
- **API tokens** (`OMADA_CLIENT_SECRET`, `DUCKDNS_TOKEN`) are encrypted at rest using AES-GCM with a per-install key stored in `.encryption-key` inside the data directory. Legacy configs encrypted with the old built-in key are migrated automatically on load.
- **Config file permissions**: `updater.conf` and `.encryption-key` are written with mode `0600` on Unix.
- **Environment overrides** (optional, for Docker/GitOps): set `OMADA_CLIENT_SECRET`, `DUCKDNS_TOKEN`, or `WEB_PASSWORD` to override file values at runtime. `WEB_PASSWORD` in the environment is plaintext and is never written back to `updater.conf`. See **Method 4: Docker** above for Compose and `.env` usage.

Example Docker CLI override using `.env` file:

```bash
cp example.env .env
chmod 600 .env
# edit .env with your secrets (OMADA_CLIENT_SECRET, DUCKDNS_TOKEN, WEB_PASSWORD)
docker run -d \
  -e DATA_DIR=/data \
  --env-file .env \
  -v omada-data:/data \
  ghcr.io/kbipinkumar/omada-duckdns-updater:latest
```

---

## Troubleshooting

- **Error: Gateway not found**: Ensure you have selected the correct Site ID in the dropdown. The gateway must be adopted in the selected site.
- **Error: Configuration is missing required fields**: Make sure you have clicked "Save Configuration" before attempting a run.
- **Forgotten Web UI Password**: Edit `updater.conf` and clear the `WEB_USERNAME` and `WEB_PASSWORD` lines, then restart the service or container.
  - Linux systemd: edit the file next to the binary (or under `$DATA_DIR`) and `systemctl restart` / `systemctl --user restart` the unit.
  - Docker: edit the file in the mounted data volume and restart the container.
  - Windows: edit `%ProgramData%\omada-duckdns-updater\updater.conf`, then `Restart-Service OmadaDuckDNSUpdater`.

---

## Disclaimer

**Please Note:** This is an LLM-generated project intended strictly for private/hobby use. It is provided "as is" without any warranties, guarantees, or official support. Please review the code and use it at your own risk before deploying it in any critical or production environments.
