package main

import (
	"strings"
	"testing"
)

func TestValidateRequestURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		opts    []ValidateURLOption
		wantErr string
		wantURL string
	}{
		{
			name:    "valid https hostname",
			raw:     "https://omada.example.com:8043",
			wantURL: "https://omada.example.com:8043",
		},
		{
			name:    "valid https ip",
			raw:     "https://192.168.1.1:8043",
			wantURL: "https://192.168.1.1:8043",
		},
		{
			name:    "missing scheme",
			raw:     "192.168.1.1:8043",
			wantErr: "invalid URL",
		},
		{
			name:    "missing host",
			raw:     "https://",
			wantErr: "URL must include scheme and host",
		},
		{
			name:    "userinfo present",
			raw:     "https://user:pass@192.168.1.1:8043",
			wantErr: "URL must not contain user info",
		},
		{
			name:    "http rejected by default",
			raw:     "http://192.168.1.1:8043",
			wantErr: "only https scheme allowed",
		},
		{
			name:    "http allowed when disabled",
			raw:     "http://192.168.1.1:8043",
			opts:    []ValidateURLOption{WithRequireHTTPS(false)},
			wantURL: "http://192.168.1.1:8043",
		},
		{
			name:    "loopback rejected when blocked",
			raw:     "https://127.0.0.1:8043",
			opts:    []ValidateURLOption{WithBlockLoopback(true)},
			wantErr: "is not allowed",
		},
		{
			name:    "loopback allowed by default",
			raw:     "https://127.0.0.1:8043",
			wantURL: "https://127.0.0.1:8043",
		},
		{
			name:    "allowlist match",
			raw:     "https://192.168.1.1:8043",
			opts:    []ValidateURLOption{WithAllowlist([]string{"192.168.1.1"})},
			wantURL: "https://192.168.1.1:8043",
		},
		{
			name:    "allowlist mismatch",
			raw:     "https://10.0.0.1:8043",
			opts:    []ValidateURLOption{WithAllowlist([]string{"192.168.1.1"})},
			wantErr: `host "10.0.0.1" not in allowlist`,
		},
		{
			name:    "invalid host shape",
			raw:     "https://-bad-host:8043",
			wantErr: "invalid host",
		},
		{
			name:    "path in host rejected by regex",
			raw:     "https://192.168.1.1:8043/extra/path",
			wantURL: "https://192.168.1.1:8043",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateRequestURL(tt.raw, tt.opts...)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("validateRequestURL(%q) = %v, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("validateRequestURL(%q) error = %q, want containing %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateRequestURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.wantURL {
				t.Fatalf("validateRequestURL(%q) = %q, want %q", tt.raw, got.String(), tt.wantURL)
			}
			if got.Path != "" {
				t.Fatalf("validateRequestURL(%q) path = %q, want empty", tt.raw, got.Path)
			}
		})
	}
}

func TestOmadaBaseURL(t *testing.T) {
	cfg := &Config{
		OmadaURL:          "https://192.168.1.1:8043",
		AllowedOmadaHosts: []string{"192.168.1.1"},
	}
	got, err := omadaBaseURL(cfg)
	if err != nil {
		t.Fatalf("omadaBaseURL() error = %v", err)
	}
	if got.String() != "https://192.168.1.1:8043" {
		t.Fatalf("omadaBaseURL() = %q, want %q", got.String(), "https://192.168.1.1:8043")
	}

	cfg.AllowedOmadaHosts = []string{"10.0.0.1"}
	if _, err := omadaBaseURL(cfg); err == nil {
		t.Fatal("omadaBaseURL() expected allowlist rejection")
	}

	cfg = &Config{OmadaURL: "https://127.0.0.1:8043"}
	if _, err := omadaBaseURL(cfg); err == nil {
		t.Fatal("omadaBaseURL() expected loopback rejection")
	}
}

func TestResolveOmadaURL(t *testing.T) {
	base, err := validateRequestURL("https://192.168.1.1:8043")
	if err != nil {
		t.Fatalf("validateRequestURL() error = %v", err)
	}

	got, err := resolveOmadaURL(base, "/openapi/v1/abc/sites?page=1&pageSize=100")
	if err != nil {
		t.Fatalf("resolveOmadaURL() error = %v", err)
	}
	want := "https://192.168.1.1:8043/openapi/v1/abc/sites?page=1&pageSize=100"
	if got != want {
		t.Fatalf("resolveOmadaURL() = %q, want %q", got, want)
	}
}
