package main

import (
	"testing"
)

// TestIsPublicIP verifies public, private, and special-use address classification.
func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip     string
		public bool
		reason string
	}{
		{"", false, "empty IP"},
		{"not-an-ip", false, "invalid IP format"},
		{"127.0.0.1", false, "loopback address"},
		{"192.168.1.1", false, "private address"},
		{"10.0.0.1", false, "private address"},
		{"100.64.0.1", false, "CG-NAT address"},
		{"fe80::1", false, "link-local address"},
		{"8.8.8.8", true, ""},
		{"2001:4860:4860::8888", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got, reason := isPublicIP(tt.ip)
			if got != tt.public {
				t.Fatalf("isPublicIP(%q) = (%v, %q), want public=%v", tt.ip, got, reason, tt.public)
			}
			if !tt.public && reason == "" {
				t.Fatalf("isPublicIP(%q) expected a rejection reason", tt.ip)
			}
		})
	}
}
