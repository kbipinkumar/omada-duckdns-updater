package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHashAndCheckPassword verifies bcrypt password hashing and verification.
func TestHashAndCheckPassword(t *testing.T) {
	hashed, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if hashed == "" {
		t.Fatal("hashPassword() returned empty hash")
	}
	if !isBcryptHash(hashed) {
		t.Fatalf("hashPassword() = %q, expected bcrypt hash", hashed)
	}
	if !checkPassword("secret", hashed) {
		t.Fatal("checkPassword() rejected valid password")
	}
	if checkPassword("wrong", hashed) {
		t.Fatal("checkPassword() accepted invalid password")
	}
}

// TestLegacySHA256Password verifies legacy salted SHA-256 password verification.
func TestLegacySHA256Password(t *testing.T) {
	salt := make([]byte, 16)
	for i := range salt {
		salt[i] = byte(i)
	}
	legacy := hexEncode(salt) + ":" + hexEncode(computeSHA256Hash("secret", salt))
	if !checkLegacySHA256Password("secret", legacy) {
		t.Fatal("checkLegacySHA256Password() rejected valid legacy password")
	}
	if checkLegacySHA256Password("wrong", legacy) {
		t.Fatal("checkLegacySHA256Password() accepted invalid password")
	}
	if !checkPassword("secret", legacy) {
		t.Fatal("checkPassword() should accept legacy SHA-256 hash")
	}
	if !needsPasswordUpgrade(legacy) {
		t.Fatal("needsPasswordUpgrade() should report legacy hash")
	}
}

// TestPlaintextPasswordRejected verifies plaintext passwords are no longer accepted.
func TestPlaintextPasswordRejected(t *testing.T) {
	if checkPassword("secret", "secret") {
		t.Fatal("checkPassword() should reject plaintext stored password")
	}
}

// TestObfuscateAndDeobfuscateToken verifies token encryption round-tripping.
func TestObfuscateAndDeobfuscateToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	resetEncryptionKeyForTests()

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

// TestPerInstallEncryptionKey verifies each data directory gets a unique encryption key.
func TestPerInstallEncryptionKey(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	t.Setenv("DATA_DIR", dir1)
	resetEncryptionKeyForTests()
	_ = obfuscateToken("shared-token")

	t.Setenv("DATA_DIR", dir2)
	resetEncryptionKeyForTests()
	_ = obfuscateToken("shared-token")

	keyPath1 := filepath.Join(dir1, ".encryption-key")
	keyPath2 := filepath.Join(dir2, ".encryption-key")

	key1, err := os.ReadFile(keyPath1)
	if err != nil {
		t.Fatalf("failed to read key1: %v", err)
	}
	key2, err := os.ReadFile(keyPath2)
	if err != nil {
		t.Fatalf("failed to read key2: %v", err)
	}

	if len(key1) != 32 {
		t.Fatalf("key1 length = %d, want 32", len(key1))
	}
	if len(key2) != 32 {
		t.Fatalf("key2 length = %d, want 32", len(key2))
	}

	if string(key1) == string(key2) {
		t.Fatal("expected different encryption keys for different data directories")
	}
}

// TestLegacyTokenMigration verifies tokens encrypted with the legacy key still decrypt.
func TestLegacyTokenMigration(t *testing.T) {
	legacyEnc := encryptTokenWithKey("legacy-token", legacyObfuscationKey)
	plain, usedLegacy := decryptToken(legacyEnc)
	if !usedLegacy {
		t.Fatal("expected legacy key usage")
	}
	if plain != "legacy-token" {
		t.Fatalf("decryptToken() = %q, want %q", plain, "legacy-token")
	}
}

// TestLegacyTokenMigrationOnLoad verifies loadConfig re-encrypts legacy tokens on load.
func TestLegacyTokenMigrationOnLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	resetEncryptionKeyForTests()

	legacyEnc := encryptTokenWithKey("client-secret", legacyObfuscationKey)
	confPath := filepath.Join(dir, "updater.conf")
	content := "OMADA_CLIENT_SECRET=" + legacyEnc + "\nDUCKDNS_TOKEN=" + legacyEnc + "\n"
	if err := os.WriteFile(confPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resetEncryptionKeyForTests()
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.OmadaClientSecret != "client-secret" {
		t.Fatalf("OmadaClientSecret = %q, want %q", cfg.OmadaClientSecret, "client-secret")
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), legacyEnc) {
		t.Fatal("expected config file to be re-encrypted with per-install key")
	}
}

// TestObfuscateTokenIdempotent verifies already-encrypted tokens are unchanged.
func TestObfuscateTokenIdempotent(t *testing.T) {
	already := "ENC:abc123"
	if got := obfuscateToken(already); got != already {
		t.Fatalf("obfuscateToken() = %q, want unchanged value", got)
	}
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
