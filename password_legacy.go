// Legacy salted SHA-256 password verification for upgrades from pre-bcrypt releases.
// New passwords are hashed with bcrypt in crypto.go.
package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// checkLegacySHA256Password verifies a legacy salted SHA-256 digest from older releases.
func checkLegacySHA256Password(password, storedHash string) bool {
	parts := strings.Split(storedHash, ":")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	hash := computeSHA256Hash(password, salt)
	if len(hash) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare(hash, expected) == 1
}

// computeSHA256Hash returns the salted SHA-256 digest used by legacy password storage.
func computeSHA256Hash(password string, salt []byte) []byte {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	return h.Sum(nil)
}
