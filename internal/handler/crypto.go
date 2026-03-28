package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltLen = 16
	keyLen  = 32
	iter    = 100_000
)

// encryptPassword encrypts plaintext using AES-256-GCM with a key derived
// from adminToken. The returned string is base64(salt + nonce + ciphertext).
func encryptPassword(plaintext, adminToken string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	key := deriveKey(adminToken, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	blob := make([]byte, 0, saltLen+len(nonce)+len(ciphertext))
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	return base64.StdEncoding.EncodeToString(blob), nil
}

// decryptPassword reverses encryptPassword.
func decryptPassword(encoded, adminToken string) (string, error) {
	blob, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(deriveKey(adminToken, blob[:saltLen]))
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	nonce := blob[saltLen : saltLen+nonceSize]
	ciphertext := blob[saltLen+nonceSize:]

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

func deriveKey(token string, salt []byte) []byte {
	return pbkdf2.Key([]byte(token), salt, iter, keyLen, sha256.New)
}
