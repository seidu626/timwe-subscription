// Package secretcrypto provides AES-256-GCM encrypt/decrypt for tenant secrets.
// The master key is read from the TENANT_SECRET_MASTER_KEY environment variable,
// which must be a base64-encoded 32-byte (256-bit) key.
// Ciphertext layout: nonce (12 bytes) || AES-GCM ciphertext.
package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

const masterKeyEnv = "TENANT_SECRET_MASTER_KEY"

// ErrMasterKeyMissing is returned when TENANT_SECRET_MASTER_KEY is not set or is invalid.
var ErrMasterKeyMissing = errors.New("TENANT_SECRET_MASTER_KEY is not set or is not a valid base64-encoded 32-byte key")

// loadMasterKey reads and validates the master key from the environment.
func loadMasterKey() ([]byte, error) {
	raw := os.Getenv(masterKeyEnv)
	if raw == "" {
		return nil, ErrMasterKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMasterKeyMissing, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: decoded length is %d, want 32", ErrMasterKeyMissing, len(key))
	}
	return key, nil
}

// Encrypt encrypts plaintext with AES-256-GCM using the master key.
// Returns nonce || ciphertext as a single byte slice.
func Encrypt(plaintext []byte) ([]byte, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a blob produced by Encrypt (nonce || ciphertext).
func Decrypt(blob []byte) ([]byte, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return nil, errors.New("secretcrypto: ciphertext too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("secretcrypto: decrypt failed: %w", err)
	}
	return plaintext, nil
}
