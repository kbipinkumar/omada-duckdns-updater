# Omada DuckDNS Updater

A lightweight, zero-dependency Go application that dynamically fetches your WAN IP addresses (both IPv4 and IPv6) from a TP-Link Omada SDN Controller and automatically updates your DuckDNS domain.

It features a beautiful, built-in Web UI for easy configuration and a live status dashboard, removing the need to mess with command-line arguments or `.env` files.

## Features
- 🚀 **Zero Dependencies**: Built entirely with Go's standard library. Single binary deployment.
- 📊 **Live Dashboard**: View the last run time, success/error status, and fetched IPs directly from the UI.
- 🔒 **Basic Authentication**: Secure your Web UI with optional username and password protection.
- ⏱️ **Dynamic Scheduling**: Update intervals are fully configurable from the Web UI—no need to restart the application.
- 🌐 **Dual Stack**: Supports updating both IPv4 and IPv6 addresses simultaneously.
- 🔍 **Site Auto-Discovery**: Automatically fetches and lists your available Omada sites to prevent misconfiguration.

---

## Prerequisites
- **Go 1.21+** (if compiling from source)
- An **Omada SDN Controller** (Hardware or Software) with an API user / Client ID & Secret configured.
- A **DuckDNS** account and domain.

---

## Installation & Deployment

You can run `omada-duckdns-updater` either directly via Systemd or containerized via Docker.

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

### Method 4: Docker Automation

We provide a wrapper script that automatically tags your Docker image with the correct Git version.

1. **Build the Image:**
   ```bash
   ./build-docker.sh
   ```

2. **Run the Container:**
   ```bash
   docker run -d \
     --name omada-ddns \
     -p 5381:5381 \
     -v $(pwd)/updater.conf:/app/updater.conf \
     omada-duckdns-updater:latest
   ```
   *(We mount `updater.conf` as a volume to ensure your configuration persists).*

Alternatively, a `docker-compose.yml` is included. You can simply run `docker compose up -d --build`.

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
5. **Save & Run:** Click **Save Configuration**, and then click **Run Now** to force an immediate update.

Check the **Status Dashboard** at the top of the page to verify that the IPs were fetched and DuckDNS was updated successfully!

---

## Troubleshooting

- **Error: Gateway not found**: Ensure you have selected the correct Site ID in the dropdown. The gateway must be adopted in the selected site.
- **Error: Configuration is missing required fields**: Make sure you have clicked "Save Configuration" before attempting a run.
- **Forgotten Web UI Password**: SSH into your server, manually edit the `updater.conf`, and clear the `WEB_USERNAME` and `WEB_PASSWORD` lines, and restart the servicedocker container.

---

## Disclaimer

**Please Note:** This is an LLM-generated project intended strictly for private/hobby use. It is provided "as is" without any warranties, guarantees, or official support. Please review the code and use it at your own risk before deploying it in any critical or production environments.
