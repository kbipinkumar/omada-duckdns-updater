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
		OmadaClientID:     "client-id",
		OmadaClientSecret: "secret",
		OmadaOmadacID:     "omadac-id",
		OmadaSiteID:       "site-123",
		DuckDNSToken:      "duck-token",
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
	if strings.Contains(summary, "client-id") || strings.Contains(summary, "duck-token") {
		t.Fatalf("describeConfig() leaked secret values: %q", summary)
	}

	incomplete := &Config{OmadaURL: "https://192.168.1.1:8043"}
	summary = describeConfig(incomplete)
	if !strings.Contains(summary, "missing=[") {
		t.Fatalf("describeConfig() = %q, want missing fields listed", summary)
	}
}

// TestInitLoggingCreatesLogFile verifies logging setup writes to the data directory.
func TestInitLoggingCreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)

	closer := initLogging()
	if closer != nil {
		defer closer.Close()
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
