package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// uiTemplate is the embedded HTML for the configuration and status dashboard.
const uiTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DuckDNS Updater Config</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --surface: #1e293b;
            --surface-light: #334155;
            --primary: #3b82f6;
            --primary-hover: #2563eb;
            --text-main: #f8fafc;
            --text-muted: #94a3b8;
            --border: #334155;
            --success: #10b981;
            --error: #ef4444;
        }

        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-main);
            display: flex;
            justify-content: center;
            align-items: flex-start;
            min-height: 100vh;
            margin: 0;
            padding: 2rem 1rem;
        }

        .container {
            background-color: var(--surface);
            padding: 2.5rem;
            border-radius: 1rem;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 10px 10px -5px rgba(0, 0, 0, 0.25);
            width: 100%;
            max-width: 600px;
            border: 1px solid var(--border);
        }

        h1 {
            margin-top: 0;
            font-size: 1.5rem;
            font-weight: 600;
            text-align: center;
            margin-bottom: 2rem;
            background: linear-gradient(to right, #60a5fa, #a78bfa);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        /* Dashboard Styles */
        .dashboard {
            background-color: var(--bg-color);
            border-radius: 0.75rem;
            padding: 1.5rem;
            margin-bottom: 2rem;
            border: 1px solid var(--border);
        }

        .dashboard h2 {
            margin-top: 0;
            font-size: 1.1rem;
            margin-bottom: 1rem;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .dash-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 1rem;
        }

        .dash-card {
            background-color: var(--surface-light);
            padding: 1rem;
            border-radius: 0.5rem;
        }

        .dash-label {
            font-size: 0.75rem;
            color: var(--text-muted);
            margin-bottom: 0.25rem;
            text-transform: uppercase;
        }

        .dash-value {
            font-size: 1rem;
            font-weight: 500;
            word-break: break-all;
        }

        .text-success { color: var(--success); }
        .text-error { color: var(--error); }
        .text-warning { color: #eab308; } /* yellow-500 */

        .dash-error {
            margin-top: 1rem;
            padding: 0.75rem;
            background-color: rgba(239, 68, 68, 0.1);
            color: var(--error);
            border: 1px solid rgba(239, 68, 68, 0.2);
            border-radius: 0.5rem;
            font-size: 0.875rem;
        }

        .dash-warning {
            margin-top: 1rem;
            padding: 0.75rem;
            background-color: rgba(234, 179, 8, 0.1);
            color: #eab308;
            border: 1px solid rgba(234, 179, 8, 0.2);
            border-radius: 0.5rem;
            font-size: 0.875rem;
        }

        /* Form Styles */
        .form-section {
            background-color: rgba(255,255,255,0.02);
            border: 1px solid var(--border);
            padding: 1.5rem;
            border-radius: 0.5rem;
            margin-bottom: 1.5rem;
        }

        .form-section-title {
            font-size: 1.1rem;
            margin-top: 0;
            margin-bottom: 1rem;
            color: var(--text-muted);
            border-bottom: 1px solid var(--border);
            padding-bottom: 0.5rem;
        }

        .form-group {
            margin-bottom: 1.25rem;
        }

        label {
            display: block;
            margin-bottom: 0.5rem;
            font-size: 0.875rem;
            font-weight: 500;
            color: var(--text-muted);
        }

        input, select {
            width: 100%;
            padding: 0.75rem;
            background-color: var(--bg-color);
            border: 1px solid var(--border);
            border-radius: 0.5rem;
            color: var(--text-main);
            font-size: 1rem;
            box-sizing: border-box;
            transition: border-color 0.2s, box-shadow 0.2s;
        }

        input:focus, select:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
        }

        button {
            width: 100%;
            padding: 0.875rem;
            background-color: var(--primary);
            color: white;
            border: none;
            border-radius: 0.5rem;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: background-color 0.2s;
        }

        button:hover {
            background-color: var(--primary-hover);
        }
        
        button:disabled {
            background-color: var(--border);
            cursor: not-allowed;
            color: var(--text-muted);
        }

        .alert {
            padding: 1rem;
            border-radius: 0.5rem;
            margin-bottom: 1.5rem;
            font-size: 0.875rem;
        }

        .alert-success {
            background-color: rgba(16, 185, 129, 0.1);
            color: var(--success);
            border: 1px solid rgba(16, 185, 129, 0.2);
        }
        
        .alert-error {
            background-color: rgba(239, 68, 68, 0.1);
            color: var(--error);
            border: 1px solid rgba(239, 68, 68, 0.2);
        }
        
        .actions {
        	display: flex;
        	gap: 1rem;
            margin-top: 1.5rem;
        }
        
        .btn-secondary {
        	background-color: transparent;
        	border: 1px solid var(--border);
        	color: var(--text-main);
        }
        
        .btn-secondary:hover:not(:disabled) {
        	background-color: var(--border);
        }
        
        .input-with-button {
            display: flex;
            gap: 0.5rem;
        }
        .input-with-button > :first-child {
            flex-grow: 1;
        }
        .input-with-button > button {
            width: auto;
            margin: 0;
            padding: 0 1rem;
            white-space: nowrap;
        }

        #wanPreviewPanel {
            margin-top: 1.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Omada DuckDNS Updater <span style="font-size: 0.8rem; font-weight: normal; color: var(--text-muted);">{{.Version}}</span></h1>

        <div class="dashboard">
            <h2>Status Dashboard</h2>
            <div class="dash-grid">
                <div class="dash-card">
                    <div class="dash-label">Last Update</div>
                    <div class="dash-value">{{if .State.LastRunTime.IsZero}}Never{{else}}{{.State.LastRunTime.Format "2006-01-02 15:04:05"}}{{end}}</div>
                </div>
                <div class="dash-card">
                    <div class="dash-label">Last Status</div>
                    <div class="dash-value {{if eq .State.LastStatus "Error"}}text-error{{else if eq .State.LastStatus "Success (OK)"}}text-success{{else if eq .State.LastStatus "Success (with warnings)"}}text-warning{{end}}">
                        {{if .State.LastStatus}}{{.State.LastStatus}}{{else}}Pending{{end}}
                    </div>
                </div>
                <div class="dash-card">
                    <div class="dash-label">Last IPv4</div>
                    <div class="dash-value">{{if .State.LastIPv4}}{{.State.LastIPv4}}{{else}}N/A{{end}}</div>
                    {{if not .State.LastIPv4UpdateTime.IsZero}}
                    <div style="font-size: 0.75rem; color: var(--text-muted); margin-top: 0.25rem;">Updated: {{.State.LastIPv4UpdateTime.Format "2006-01-02 15:04:05"}}</div>
                    {{end}}
                </div>
                <div class="dash-card">
                    <div class="dash-label">Last IPv6</div>
                    <div class="dash-value">{{if .State.LastIPv6}}{{.State.LastIPv6}}{{else}}N/A{{end}}</div>
                    {{if not .State.LastIPv6UpdateTime.IsZero}}
                    <div style="font-size: 0.75rem; color: var(--text-muted); margin-top: 0.25rem;">Updated: {{.State.LastIPv6UpdateTime.Format "2006-01-02 15:04:05"}}</div>
                    {{end}}
                </div>
            </div>
            {{if .State.LastError}}
            <div class="dash-error">
                <strong>Error Details:</strong> {{.State.LastError}}
            </div>
            {{end}}
            {{if .State.LastWarning}}
            <div class="dash-warning">
                <strong>Warning Details:</strong> {{.State.LastWarning}}
            </div>
            {{end}}
        </div>
        
        {{if .Message}}
        <div class="alert {{if .Error}}alert-error{{else}}alert-success{{end}}">
            {{.Message}}
        </div>
        {{end}}

        <form method="POST" action="/save" id="configForm">
            
            <div class="form-section">
                <h3 class="form-section-title">Omada Configuration</h3>
                <div class="form-group">
                    <label for="OmadaURL">Omada Controller URL</label>
                    <input type="url" id="OmadaURL" name="OmadaURL" value="{{.Config.OmadaURL}}" placeholder="https://192.168.1.1:8043" required>
                </div>
                
                <div class="form-group">
                    <label for="OmadaClientID">Omada Client ID</label>
                    <input type="text" id="OmadaClientID" name="OmadaClientID" value="{{.Config.OmadaClientID}}" required>
                </div>
                
                <div class="form-group">
                    <label for="OmadaClientSecret">Omada Client Secret</label>
                    <input type="password" id="OmadaClientSecret" name="OmadaClientSecret" value="{{.Config.OmadaClientSecret}}" required>
                </div>
                
                <div class="form-group">
                    <label for="OmadaOmadacID">Omada Omadac ID</label>
                    <input type="text" id="OmadaOmadacID" name="OmadaOmadacID" value="{{.Config.OmadaOmadacID}}" required>
                </div>
                
                <div class="form-group">
                    <label for="OmadaSiteID">Omada Site</label>
                    <div class="input-with-button" id="siteSelectionArea">
                        <input type="text" id="OmadaSiteID" name="OmadaSiteID" value="{{.Config.OmadaSiteID}}" required placeholder="Site ID">
                        <button type="button" class="btn-secondary" id="fetchSitesBtn" onclick="fetchSites()">Fetch</button>
                    </div>
                </div>
            </div>

            <div class="form-section">
                <h3 class="form-section-title">DuckDNS Configuration</h3>
                <div class="form-group">
                    <label for="DuckDNSToken">DuckDNS Token</label>
                    <input type="password" id="DuckDNSToken" name="DuckDNSToken" value="{{.Config.DuckDNSToken}}" required>
                </div>

                <div class="form-group">
                    <label>DuckDNS Domains (Up to 5)</label>
                    {{$domains := .Config.DuckDNSDomains}}
                    <input type="text" name="DuckDNSDomain1" value="{{index $domains 0}}" placeholder="example1.duckdns.org" required style="margin-bottom: 0.5rem;">
                    <input type="text" name="DuckDNSDomain2" value="{{index $domains 1}}" placeholder="example2.duckdns.org (Optional)" style="margin-bottom: 0.5rem;">
                    <input type="text" name="DuckDNSDomain3" value="{{index $domains 2}}" placeholder="example3.duckdns.org (Optional)" style="margin-bottom: 0.5rem;">
                    <input type="text" name="DuckDNSDomain4" value="{{index $domains 3}}" placeholder="example4.duckdns.org (Optional)" style="margin-bottom: 0.5rem;">
                    <input type="text" name="DuckDNSDomain5" value="{{index $domains 4}}" placeholder="example5.duckdns.org (Optional)">
                </div>

                <div class="form-group">
                    <label>IP Protocols to Update</label>
                    <div style="display: flex; gap: 1rem; align-items: center; margin-top: 0.5rem;">
                        <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; font-weight: normal;">
                            <input type="checkbox" name="UpdateIPv4" {{if .Config.UpdateIPv4}}checked{{end}} style="width: auto; margin: 0;"> Update IPv4
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; font-weight: normal;">
                            <input type="checkbox" name="UpdateIPv6" {{if .Config.UpdateIPv6}}checked{{end}} style="width: auto; margin: 0;"> Update IPv6
                        </label>
                    </div>
                </div>
            </div>

            <div class="form-section">
                <h3 class="form-section-title">System Settings</h3>
                <div class="form-group">
                    <label for="UpdateInterval">Update Interval (Minutes)</label>
                    <input type="number" id="UpdateInterval" name="UpdateInterval" value="{{.Config.UpdateInterval}}" min="1" required>
                </div>
                <div class="form-group">
                    <label for="WebUsername">Web UI Username (Optional, for Basic Auth)</label>
                    <input type="text" id="WebUsername" name="WebUsername" value="{{.Config.WebUsername}}">
                </div>
                <div class="form-group">
                    <label for="WebPassword">Web UI Password (Optional)</label>
                    <input type="password" id="WebPassword" name="WebPassword" placeholder="{{if .Config.WebPassword}}(unchanged){{end}}">
                </div>
            </div>

            <div id="wanPreviewPanel" class="dashboard" style="display: none;">
                <h2>WAN IP Preview</h2>
                <p style="color: var(--text-muted); font-size: 0.875rem; margin: 0 0 1rem 0;">
                    Review the WAN addresses retrieved from your Omada controller before saving configuration.
                </p>
                <div class="dash-grid">
                    <div class="dash-card">
                        <div class="dash-label">WAN IPv4</div>
                        <div class="dash-value" id="previewIPv4">N/A</div>
                    </div>
                    <div class="dash-card">
                        <div class="dash-label">WAN IPv6</div>
                        <div class="dash-value" id="previewIPv6">N/A</div>
                    </div>
                </div>
                <div id="previewWarnings" class="dash-warning" style="display: none;"></div>
                <div id="previewError" class="dash-error" style="display: none;"></div>
            </div>

			<div class="actions">
            {{if .IsFirstRun}}
            	<button type="button" class="btn-secondary" id="verifyBtn" onclick="verifyConnection()">Verify Connection</button>
            	<button type="submit" id="confirmSaveBtn" style="display: none;">Confirm &amp; Save Configuration</button>
            {{else}}
            	<button type="submit">Save Configuration</button>
            	<button type="button" class="btn-secondary" onclick="runNow()">Run Now</button>
            {{end}}
            </div>
        </form>
    </div>
    
    <script>
    const isFirstRun = {{if .IsFirstRun}}true{{else}}false{{end}};
    let verifyCompleted = false;

    function resetVerifyState() {
        verifyCompleted = false;
        const confirmBtn = document.getElementById('confirmSaveBtn');
        if (confirmBtn) {
            confirmBtn.style.display = 'none';
        }
        const panel = document.getElementById('wanPreviewPanel');
        if (panel) {
            panel.style.display = 'none';
        }
        const warnings = document.getElementById('previewWarnings');
        const error = document.getElementById('previewError');
        if (warnings) {
            warnings.style.display = 'none';
            warnings.textContent = '';
        }
        if (error) {
            error.style.display = 'none';
            error.textContent = '';
        }
    }

    function setupFirstRunListeners() {
        if (!isFirstRun) return;
        const fields = ['OmadaURL', 'OmadaClientID', 'OmadaClientSecret', 'OmadaOmadacID', 'OmadaSiteID'];
        const form = document.getElementById('configForm');
        if (form) {
            const handler = function(e) {
                if (e.target && fields.indexOf(e.target.id) !== -1) {
                    resetVerifyState();
                }
            };
            form.addEventListener('input', handler);
            form.addEventListener('change', handler);
            form.addEventListener('submit', function(e) {
                if (!verifyCompleted) {
                    e.preventDefault();
                    alert('Please verify the connection and review WAN IPs before saving.');
                }
            });
        }
    }

    function verifyConnection() {
        const btn = document.getElementById('verifyBtn');
        const url = document.getElementById('OmadaURL').value;
        const clientId = document.getElementById('OmadaClientID').value;
        const clientSecret = document.getElementById('OmadaClientSecret').value;
        const omadacId = document.getElementById('OmadaOmadacID').value;
        const siteId = document.getElementById('OmadaSiteID').value;
        const updateIPv4 = document.querySelector('input[name="UpdateIPv4"]').checked;
        const updateIPv6 = document.querySelector('input[name="UpdateIPv6"]').checked;

        if (!url || !clientId || !clientSecret || !omadacId || !siteId) {
            alert('Please fill in all Omada fields including Site ID before verifying.');
            return;
        }

        btn.disabled = true;
        btn.innerText = 'Verifying...';
        resetVerifyState();

        const payload = {
            OmadaURL: url,
            OmadaClientID: clientId,
            OmadaClientSecret: clientSecret,
            OmadaOmadacID: omadacId,
            OmadaSiteID: siteId,
            UpdateIPv4: updateIPv4,
            UpdateIPv6: updateIPv6
        };

        fetch('/api/wan', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(payload)
        })
        .then(r => {
            if (!r.ok) return r.text().then(t => { throw new Error(t); });
            return r.json();
        })
        .then(data => {
            const panel = document.getElementById('wanPreviewPanel');
            const ipv4El = document.getElementById('previewIPv4');
            const ipv6El = document.getElementById('previewIPv6');
            const warningsEl = document.getElementById('previewWarnings');
            const errorEl = document.getElementById('previewError');

            panel.style.display = 'block';
            ipv4El.textContent = data.ipv4 || 'N/A';
            ipv6El.textContent = data.ipv6 || 'N/A';

            if (data.warnings && data.warnings.length > 0) {
                warningsEl.style.display = 'block';
                warningsEl.textContent = '';
                const wLabel = document.createElement('strong');
                wLabel.textContent = 'Warnings:';
                warningsEl.appendChild(wLabel);
                warningsEl.appendChild(document.createTextNode(' ' + data.warnings.join(' | ')));
            }

            verifyCompleted = true;
            const confirmBtn = document.getElementById('confirmSaveBtn');
            if (confirmBtn) {
                confirmBtn.style.display = 'block';
            }

            btn.disabled = false;
            btn.innerText = 'Verify Connection';
        })
        .catch(e => {
            const panel = document.getElementById('wanPreviewPanel');
            const errorEl = document.getElementById('previewError');
            panel.style.display = 'block';
            errorEl.style.display = 'block';
            errorEl.textContent = '';
            const eLabel = document.createElement('strong');
            eLabel.textContent = 'Verification failed:';
            errorEl.appendChild(eLabel);
            errorEl.appendChild(document.createTextNode(' ' + e.message));
            btn.disabled = false;
            btn.innerText = 'Verify Connection';
        });
    }

    document.addEventListener('DOMContentLoaded', setupFirstRunListeners);

    function runNow() {
        const form = document.getElementById('configForm');
        const formData = new URLSearchParams(new FormData(form));
        
        fetch('/save', {
            method: 'POST',
            body: formData,
            headers: {'Content-Type': 'application/x-www-form-urlencoded'}
        }).then(() => {
            return fetch('/run', {method: 'POST'});
        })
    	.then(r => {
            if (!r.ok) return r.text().then(t => { throw new Error(t); });
            return r.text();
        })
    	.then(msg => {
            alert(msg);
            window.location.reload();
        })
    	.catch(e => alert(e.message));
    }

    function fetchSites() {
        const btn = document.getElementById('fetchSitesBtn');
        const url = document.getElementById('OmadaURL').value;
        const clientId = document.getElementById('OmadaClientID').value;
        const clientSecret = document.getElementById('OmadaClientSecret').value;
        const omadacId = document.getElementById('OmadaOmadacID').value;
        const currentSiteId = document.getElementById('OmadaSiteID').value;

        if (!url || !clientId || !clientSecret || !omadacId) {
            alert("Please fill in the Omada URL, Client ID, Client Secret, and Omadac ID first.");
            return;
        }

        btn.disabled = true;
        btn.innerText = "Fetching...";

        const payload = {
            OmadaURL: url,
            OmadaClientID: clientId,
            OmadaClientSecret: clientSecret,
            OmadaOmadacID: omadacId
        };

        fetch('/api/sites', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(payload)
        })
        .then(r => {
            if (!r.ok) return r.text().then(t => { throw new Error(t); });
            return r.json();
        })
        .then(sites => {
            if (!sites || sites.length === 0) {
                alert("No sites found.");
                btn.disabled = false;
                btn.innerText = "Fetch";
                return;
            }

            const select = document.createElement('select');
            select.id = 'OmadaSiteID';
            select.name = 'OmadaSiteID';
            select.required = true;

            sites.forEach(site => {
                const opt = document.createElement('option');
                opt.value = site.siteId;
                opt.innerText = site.name + " (" + site.siteId + ")";
                if (site.siteId === currentSiteId) opt.selected = true;
                select.appendChild(opt);
            });

            const oldInput = document.getElementById('OmadaSiteID');
            oldInput.parentNode.replaceChild(select, oldInput);
            
            btn.innerText = "Refresh";
            btn.disabled = false;
        })
        .catch(e => {
            alert("Failed to fetch sites: " + e.message);
            btn.disabled = false;
            btn.innerText = "Fetch";
        });
    }
    </script>
</body>
</html>
`

// tmpl is the parsed web UI template used by HTTP handlers.
var tmpl = template.Must(template.New("ui").Parse(uiTemplate))

// PageData supplies template variables for the configuration page.
type PageData struct {
	Config     *Config
	Message    string
	Error      bool
	State      StateView
	Version    string
	IsFirstRun bool
}

// WanPreviewResponse is the JSON payload returned by the WAN preview API.
type WanPreviewResponse struct {
	IPv4       string   `json:"ipv4"`
	IPv6       string   `json:"ipv6"`
	IPv4Public bool     `json:"ipv4Public"`
	IPv6Public bool     `json:"ipv6Public"`
	Warnings   []string `json:"warnings"`
}

// basicAuth wraps an HTTP handler with optional web UI basic authentication.
func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config, err := loadConfig()
		if err != nil {
			http.Error(w, "Internal Server Error: failed to load configuration", http.StatusInternalServerError)
			return
		}
		if config.WebUsername != "" && webPasswordForAuth(config) != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != config.WebUsername || !verifyWebPassword(pass, config.WebPassword) {
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if os.Getenv("WEB_PASSWORD") == "" && needsPasswordUpgrade(config.WebPassword) {
				if hashed, hashErr := hashPassword(pass); hashErr == nil {
					config.WebPassword = hashed
					_ = saveConfig(config)
				}
			}
		}
		next(w, r)
	}
}

// handleIndex renders the configuration page and status dashboard.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	config, err := loadConfig()
	if err != nil {
		config = &Config{UpdateInterval: 5}
	}

	data := PageData{
		Config:     config,
		State:      snapshotState(),
		Version:    version,
		IsFirstRun: isFirstRun(config),
	}
	tmpl.Execute(w, data)
}

// handleSave persists configuration submitted from the web UI form.
func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	
	interval, _ := strconv.Atoi(r.FormValue("UpdateInterval"))
	if interval <= 0 {
		interval = 5
	}

	var domains []string
	for i := 1; i <= 5; i++ {
		d := strings.TrimSpace(r.FormValue(fmt.Sprintf("DuckDNSDomain%d", i)))
		if d != "" {
			domains = append(domains, d)
		}
	}
	duckDNSDomain := strings.Join(domains, ",")

	newWebPassword := r.FormValue("WebPassword")
	oldConfig, _ := loadConfig()
	finalWebPassword := ""
	if oldConfig != nil {
		finalWebPassword = oldConfig.WebPassword
	}
	if newWebPassword != "" {
		if hashed, err := hashPassword(newWebPassword); err == nil {
			finalWebPassword = hashed
		}
	}

	config := &Config{
		OmadaURL:          r.FormValue("OmadaURL"),
		OmadaClientID:     r.FormValue("OmadaClientID"),
		OmadaClientSecret: r.FormValue("OmadaClientSecret"),
		OmadaOmadacID:     r.FormValue("OmadaOmadacID"),
		OmadaSiteID:       r.FormValue("OmadaSiteID"),
		DuckDNSToken:      r.FormValue("DuckDNSToken"),
		DuckDNSDomain:     duckDNSDomain,
		UpdateIPv4:        r.FormValue("UpdateIPv4") == "on",
		UpdateIPv6:        r.FormValue("UpdateIPv6") == "on",
		UpdateInterval:    interval,
		WebUsername:       r.FormValue("WebUsername"),
		WebPassword:       finalWebPassword,
	}

	err := saveConfig(config)

	data := PageData{
		Config:     config,
		State:      snapshotState(),
		Version:    version,
		IsFirstRun: isFirstRun(config),
	}
	
	if err != nil {
		data.Message = "Failed to save configuration: " + err.Error()
		data.Error = true
	} else {
		data.Message = "Configuration saved successfully! It will be used on the next run."
		data.Error = false
	}
	
	tmpl.Execute(w, data)
}

// handleRun triggers an immediate DuckDNS update from the web UI.
func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	err := runUpdate(true)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error running update: %v", err)
		return
	}
	fmt.Fprintf(w, "Update successful!")
}

// handleApiSites returns available Omada sites for the site picker UI.
func handleApiSites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var reqConfig Config
	if err := json.NewDecoder(r.Body).Decode(&reqConfig); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid request: %v", err)
		return
	}
	
	token, err := getOmadaToken(&reqConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to authenticate with Omada: %v", err)
		return
	}
	
	sites, err := fetchSites(&reqConfig, token)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to fetch sites: %v", err)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sites)
}

// handleApiWan previews gateway WAN IPs before first-run configuration is saved.
func handleApiWan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var reqConfig Config
	if err := json.NewDecoder(r.Body).Decode(&reqConfig); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid request: %v", err)
		return
	}

	if reqConfig.OmadaURL == "" || reqConfig.OmadaClientID == "" ||
		reqConfig.OmadaClientSecret == "" || reqConfig.OmadaOmadacID == "" ||
		reqConfig.OmadaSiteID == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Missing required Omada fields (URL, Client ID, Secret, Omadac ID, Site ID)")
		return
	}

	token, err := getOmadaToken(&reqConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to authenticate with Omada: %v", err)
		return
	}

	ipv4, ipv6, err := getGatewayWAN(&reqConfig, token)
	if err != nil {
		if errors.Is(err, ErrWANDown) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "WAN interface is down or has no IP from ISP")
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to fetch WAN status: %v", err)
		return
	}

	response := WanPreviewResponse{
		IPv4: ipv4,
		IPv6: ipv6,
	}

	if ipv4 != "" {
		if ok, reason := isPublicIP(ipv4); ok {
			response.IPv4Public = true
		} else {
			response.Warnings = append(response.Warnings, fmt.Sprintf("IPv4 ignored (%s)", reason))
		}
	}

	if ipv6 != "" {
		if ok, reason := isPublicIP(ipv6); ok {
			response.IPv6Public = true
		} else {
			response.Warnings = append(response.Warnings, fmt.Sprintf("IPv6 ignored (%s)", reason))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// startWebServer serves the configuration UI until ctx is cancelled.
func startWebServer(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", basicAuth(handleIndex))
	mux.HandleFunc("/save", basicAuth(handleSave))
	mux.HandleFunc("/run", basicAuth(handleRun))
	mux.HandleFunc("/api/sites", basicAuth(handleApiSites))
	mux.HandleFunc("/api/wan", basicAuth(handleApiWan))

	port := "5381"
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Web server shutdown error: %v", err)
		}
	}()

	log.Printf("Starting Web UI on http://localhost:%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Web server error: %v", err)
	}
}
