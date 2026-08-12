package main

import (
	"strings"
	"testing"
)

// TestHashAndCheckPassword verifies legacy salted SHA-256 password verification.
func TestHashAndCheckPassword(t *testing.T) {
	hashed, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if hashed == "" {
		t.Fatal("hashPassword() returned empty hash")
	}
	if !strings.Contains(hashed, ":") {
		t.Fatalf("hashPassword() = %q, expected salt:hash format", hashed)
	}
	if !checkPassword("secret", hashed) {
		t.Fatal("checkPassword() rejected valid password")
	}
	if checkPassword("wrong", hashed) {
		t.Fatal("checkPassword() accepted invalid password")
	}
}

// TestObfuscateAndDeobfuscateToken verifies token obfuscation round-tripping.
func TestObfuscateAndDeobfuscateToken(t *testing.T) {
	original := "my-secret-token"
	obfuscated := obfuscateToken(original)
	if obfuscated == original {
		t.Fatal("obfuscateToken() did not change token")
	}
	if !strings.HasPrefix(obfuscated, "ENC:") {
		t.Fatalf("obfuscateToken() = %q, want ENC: prefix", obfuscated)
	}

	restored := deobfuscateToken(obfuscated)
	if restored != original {
		t.Fatalf("deobfuscateToken() = %q, want %q", restored, original)
	}
}

// TestObfuscateTokenIdempotent verifies already-obfuscated tokens are unchanged.
func TestObfuscateTokenIdempotent(t *testing.T) {
	already := "ENC:abc123"
	if got := obfuscateToken(already); got != already {
		t.Fatalf("obfuscateToken() = %q, want unchanged value", got)
	}
}
