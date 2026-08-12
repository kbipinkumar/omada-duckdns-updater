package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const encryptionKeyFile = ".encryption-key"

// legacyObfuscationKey was used before per-install keys; kept for one-time migration.
var legacyObfuscationKey = []byte("omada-duckdns-updater-obfuscate-") // 32 bytes

var (
	encryptionKey     []byte
	encryptionKeyErr  error
	encryptionKeyOnce sync.Once
)

// hashPassword returns a bcrypt hash suitable for storing web UI passwords.
func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// checkPassword verifies password against a bcrypt hash or a legacy salted SHA-256 digest.
func checkPassword(password, storedHash string) bool {
	if storedHash == "" {
		return password == ""
	}
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	return checkLegacySHA256Password(password, storedHash)
}

// needsPasswordUpgrade reports whether storedHash uses a legacy format that should be re-hashed.
func needsPasswordUpgrade(storedHash string) bool {
	if storedHash == "" {
		return false
	}
	return !isBcryptHash(storedHash)
}

// isBcryptHash reports whether storedHash looks like a bcrypt hash.
func isBcryptHash(storedHash string) bool {
	return strings.HasPrefix(storedHash, "$2a$") ||
		strings.HasPrefix(storedHash, "$2b$") ||
		strings.HasPrefix(storedHash, "$2y$")
}

func encryptionKeyPath() string {
	dir := getDataDir()
	if dir != "" {
		return filepath.Join(dir, encryptionKeyFile)
	}
	return encryptionKeyFile
}

// getEncryptionKey returns the per-install AES key, creating it on first use.
func getEncryptionKey() ([]byte, error) {
	encryptionKeyOnce.Do(func() {
		path := encryptionKeyPath()
		info, statErr := os.Stat(path)
		if statErr == nil {
			mode := info.Mode()
			if mode&0077 != 0 {
				encryptionKeyErr = fmt.Errorf("encryption key file %s has insecure permissions %o (expected 0600)", path, mode.Perm())
				return
			}
		}
		if data, err := os.ReadFile(path); err == nil {
			if len(data) == 32 {
				encryptionKey = data
				return
			}
			encryptionKeyErr = fmt.Errorf("invalid encryption key length in %s", path)
			return
		} else if !os.IsNotExist(err) {
			encryptionKeyErr = err
			return
		}

		if err := ensureDataDir(); err != nil {
			encryptionKeyErr = err
			return
		}

		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			encryptionKeyErr = err
			return
		}

		if err := os.WriteFile(path, key, 0600); err != nil {
			encryptionKeyErr = err
			return
		}
		encryptionKey = key
	})
	return encryptionKey, encryptionKeyErr
}

// resetEncryptionKeyForTests clears the cached encryption key between tests.
func resetEncryptionKeyForTests() {
	encryptionKey = nil
	encryptionKeyErr = nil
	encryptionKeyOnce = sync.Once{}
}

// obfuscateToken encrypts token for storage in updater.conf with an ENC: prefix.
func obfuscateToken(token string) string {
	if token == "" || strings.HasPrefix(token, "ENC:") {
		return token
	}
	key, err := getEncryptionKey()
	if err != nil {
		return token
	}
	return encryptTokenWithKey(token, key)
}

// deobfuscateToken decrypts a token previously stored by obfuscateToken.
func deobfuscateToken(token string) string {
	plain, _ := decryptToken(token)
	return plain
}

// decryptToken decrypts an ENC:-prefixed token and reports whether the legacy key was used.
func decryptToken(token string) (string, bool) {
	if !strings.HasPrefix(token, "ENC:") {
		return token, false
	}

	if key, err := getEncryptionKey(); err == nil {
		if plain := decryptTokenWithKey(token, key); plain != token {
			return plain, false
		}
	}

	if plain := decryptTokenWithKey(token, legacyObfuscationKey); plain != token {
		return plain, true
	}
	return token, false
}

func encryptTokenWithKey(token string, key []byte) string {
	block, err := aes.NewCipher(key)
	if err != nil {
		return token
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return token
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return token
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(token), nil)
	return "ENC:" + base64.StdEncoding.EncodeToString(ciphertext)
}

func decryptTokenWithKey(token string, key []byte) string {
	if !strings.HasPrefix(token, "ENC:") {
		return token
	}
	encodedStr := strings.TrimPrefix(token, "ENC:")
	enc, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return token
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return token
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return token
	}
	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize {
		return token
	}
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return token
	}
	return string(plaintext)
}
