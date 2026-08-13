package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDescribeConfig verifies safe configuration summaries for troubleshooting logs.
func TestDescribeConfig(t *testing.T) {
	complete := &Config{
		OmadaURL:          "https://192.168.1.1:8043",
		OmadaClientID:     "client-id-value",
		OmadaClientSecret: "client-secret-value",
		OmadaOmadacID:     "omadac-id-value",
		OmadaSiteID:       "site-123",
		DuckDNSToken:      "duck-token-value",
		DuckDNSDomain:     "example.duckdns.org",
		UpdateIPv4:        true,
		UpdateInterval:    10,
	}

	summary := describeConfig(complete)
	for _, want := range []string{
		`omada_site_id="site-123"`,
		"omada_client_secret=set",
		"duckdns_token=set",
		`duckdns_domain="example.duckdns.org"`,
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("describeConfig() = %q, want substring %q", summary, want)
		}
	}
	for _, secret := range []string{
		"client-id-value",
		"client-secret-value",
		"omadac-id-value",
		"duck-token-value",
	} {
		if strings.Contains(summary, secret) {
			t.Fatalf("describeConfig() leaked value %q in summary: %q", secret, summary)
		}
	}

	incomplete := &Config{OmadaURL: "https://192.168.1.1:8043"}
	summary = describeConfig(incomplete)
	if !strings.Contains(summary, "missing=[") {
		t.Fatalf("describeConfig() = %q, want missing fields listed", summary)
	}
}

// TestSanitizeLogMessage verifies control characters are escaped in log output.
func TestSanitizeLogMessage(t *testing.T) {
	got := sanitizeLogMessage("line1\nforged\rrecord")
	if got != `line1\nforged\rrecord` {
		t.Fatalf("sanitizeLogMessage() = %q", got)
	}
}

// TestSanitizeURLForLog verifies URL userinfo is removed before logging.
func TestSanitizeURLForLog(t *testing.T) {
	got := sanitizeURLForLog("https://admin:secret@192.168.1.1:8043")
	if strings.Contains(got, "admin") || strings.Contains(got, "secret") {
		t.Fatalf("sanitizeURLForLog() leaked credentials: %q", got)
	}
	if got != "https://192.168.1.1:8043" {
		t.Fatalf("sanitizeURLForLog() = %q, want host-only URL", got)
	}
}

// TestInitLoggingCreatesLogFile verifies logging setup writes to the data directory.
func TestInitLoggingCreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)

	closer := initLogging()
	if closer != nil {
		t.Cleanup(func() {
			if err := closer.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
	}

	logInfo("test log entry")

	logPath := filepath.Join(dir, logFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "logging initialized") {
		t.Fatalf("expected initialization message in log, got:\n%s", content)
	}
	if !strings.Contains(content, "test log entry") {
		t.Fatalf("expected test log entry in log, got:\n%s", content)
	}
}
