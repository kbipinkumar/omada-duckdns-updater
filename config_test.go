package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLoadSaveConfig verifies config round-tripping and obfuscated secret storage.
func TestLoadSaveConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)

	cfg := &Config{
		OmadaURL:          "https://192.168.1.1:8043",
		OmadaClientID:     "client-id",
		OmadaClientSecret: "client-secret",
		OmadaOmadacID:     "omadac-id",
		OmadaSiteID:       "site-id",
		DuckDNSToken:      "duck-token",
		DuckDNSDomain:     "example.duckdns.org",
		UpdateIPv4:        true,
		UpdateIPv6:        false,
		UpdateInterval:    10,
		WebUsername:       "admin",
		WebPassword:       "hashed-password",
	}

	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig() error = %v", err)
	}

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if loaded.OmadaURL != cfg.OmadaURL {
		t.Fatalf("OmadaURL = %q, want %q", loaded.OmadaURL, cfg.OmadaURL)
	}
	if loaded.OmadaClientSecret != cfg.OmadaClientSecret {
		t.Fatalf("OmadaClientSecret = %q, want %q", loaded.OmadaClientSecret, cfg.OmadaClientSecret)
	}
	if loaded.DuckDNSToken != cfg.DuckDNSToken {
		t.Fatalf("DuckDNSToken = %q, want %q", loaded.DuckDNSToken, cfg.DuckDNSToken)
	}
	if loaded.UpdateInterval != cfg.UpdateInterval {
		t.Fatalf("UpdateInterval = %d, want %d", loaded.UpdateInterval, cfg.UpdateInterval)
	}
	if loaded.WebPassword != cfg.WebPassword {
		t.Fatalf("WebPassword = %q, want %q", loaded.WebPassword, cfg.WebPassword)
	}

	path := filepath.Join(dir, "updater.conf")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0600 {
			t.Fatalf("config file mode = %o, want 0600", info.Mode().Perm())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	for _, part := range []string{
		"OMADA_CLIENT_SECRET=ENC:",
		"DUCKDNS_TOKEN=ENC:",
	} {
		if !strings.Contains(content, part) {
			t.Fatalf("expected %q in config file, got:\n%s", part, content)
		}
	}
	for _, secret := range []string{cfg.OmadaClientSecret, cfg.DuckDNSToken} {
		if strings.Contains(content, secret) {
			t.Fatalf("config file contains plaintext secret %q", secret)
		}
	}
}

// TestConfigIsComplete verifies required-field detection for update cycles.
func TestConfigIsComplete(t *testing.T) {
	complete := &Config{
		OmadaURL:      "https://192.168.1.1:8043",
		OmadaSiteID:   "site-id",
		DuckDNSToken:  "duck-token",
		DuckDNSDomain: "example.duckdns.org",
	}
	if !configIsComplete(complete) {
		t.Fatalf("expected complete config to be ready for updates")
	}

	missingSite := *complete
	missingSite.OmadaSiteID = ""
	if configIsComplete(&missingSite) {
		t.Fatalf("expected config without site ID to be incomplete")
	}
	if got := missingConfigFields(&missingSite); len(got) != 1 || got[0] != "Omada Site ID" {
		t.Fatalf("missingConfigFields() = %#v, want [Omada Site ID]", got)
	}

	missingToken := *complete
	missingToken.DuckDNSToken = ""
	if got := missingConfigFields(&missingToken); len(got) != 1 || got[0] != "DuckDNS Token" {
		t.Fatalf("missingConfigFields() = %#v, want [DuckDNS Token]", got)
	}
}

// TestDuckDNSDomains verifies comma-separated domains are split into five slots.
func TestDuckDNSDomains(t *testing.T) {
	cfg := &Config{DuckDNSDomain: "a.example.com,b.example.com"}
	domains := cfg.DuckDNSDomains()
	if domains[0] != "a.example.com" || domains[1] != "b.example.com" {
		t.Fatalf("DuckDNSDomains() = %#v", domains)
	}
	if domains[2] != "" {
		t.Fatalf("expected empty slot at index 2, got %q", domains[2])
	}
}
