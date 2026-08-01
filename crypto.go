package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// A hardcoded key for obfuscation. This is not secure against determined attackers,
// but it prevents casual snooping of plaintext tokens in the config file.
var obfuscationKey = []byte("omada-duckdns-updater-obfuscate-") // 32 bytes

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	hash := computeSHA256Hash(password, salt)
	return fmt.Sprintf("%s:%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

func checkPassword(password, storedHash string) bool {
	if storedHash == "" {
		return password == ""
	}
	parts := strings.Split(storedHash, ":")
	if len(parts) != 2 {
		return password == storedHash // fallback for unhashed passwords previously stored
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	hash := computeSHA256Hash(password, salt)
	return hex.EncodeToString(hash) == parts[1]
}

func computeSHA256Hash(password string, salt []byte) []byte {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	return h.Sum(nil)
}

func obfuscateToken(token string) string {
	if token == "" || strings.HasPrefix(token, "ENC:") {
		return token
	}
	block, err := aes.NewCipher(obfuscationKey)
	if err != nil {
		return token // fallback
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

func deobfuscateToken(token string) string {
	if !strings.HasPrefix(token, "ENC:") {
		return token
	}
	encodedStr := strings.TrimPrefix(token, "ENC:")
	enc, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return token
	}
	block, err := aes.NewCipher(obfuscationKey)
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
