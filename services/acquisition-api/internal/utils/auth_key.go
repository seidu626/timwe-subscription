package utils

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// pad applies PKCS7 padding to data so its length is a multiple of blockSize.
func pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// encryptECB encrypts plaintext using AES in ECB mode.
// It checks that key length is one of 16, 24, or 32 bytes.
func encryptECB(plaintext, key []byte) ([]byte, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, errors.New("key length must be 16, 24, or 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	padded := pad(plaintext, blockSize)

	ciphertext := make([]byte, len(padded))
	for start := 0; start < len(padded); start += blockSize {
		block.Encrypt(ciphertext[start:start+blockSize], padded[start:start+blockSize])
	}

	return ciphertext, nil
}

// GenerateAuthKey creates a Base64-encoded AES-ECB token of the form:
//
//	partnerServiceID#timestamp
//
// It mirrors the Python logic: pads with PKCS7, encrypts with AES-ECB,
// and returns the Base64 string.
func GenerateAuthKey(partnerServiceID, presharedKey string) (string, error) {
	return GenerateAuthKeyWithTimestamp(partnerServiceID, presharedKey, time.Now().UnixMilli())
}

// GenerateAuthKeyWithTimestamp is a deterministic variant for testing/parity checks with Python.
// It uses AES-ECB with PKCS7 padding exactly as Crypto.Util.Padding.pad.
func GenerateAuthKeyWithTimestamp(partnerServiceID, presharedKey string, timestampMs int64) (string, error) {
	// Build the token string using the provided timestamp (milliseconds)
	token := fmt.Sprintf("%s#%d", partnerServiceID, timestampMs)
	tokenBytes := []byte(token)

	// Convert preshared key to bytes
	keyBytes := []byte(presharedKey)

	// Encrypt the padded token using AES-ECB helper
	cipherBytes, err := encryptECB(tokenBytes, keyBytes)
	if err != nil {
		return "", err
	}

	// Base64-encode the ciphertext and return
	return base64.StdEncoding.EncodeToString(cipherBytes), nil
}

// Cached auth key handling (24h TTL)

type cachedAuthKey struct {
	value       string
	generatedAt time.Time
}

var (
	authKeyCache = struct {
		mu sync.RWMutex
		m  map[string]cachedAuthKey
	}{
		m: make(map[string]cachedAuthKey),
	}

	// defaultAuthKeyTTL defines how long a generated auth key remains valid
	defaultAuthKeyTTL = 24 * time.Hour
)

// GetCachedAuthKey returns a cached auth key for the given partnerServiceID + presharedKey
// pair if it exists and has not expired. Otherwise, it generates a new one,
// stores it in the cache, and returns it.
func GetCachedAuthKey(partnerServiceID, presharedKey string) (string, error) {
	cacheKey := partnerServiceID + "|" + presharedKey

	// Fast path: read lock to check existing and valid
	authKeyCache.mu.RLock()
	entry, ok := authKeyCache.m[cacheKey]
	authKeyCache.mu.RUnlock()
	if ok {
		if time.Since(entry.generatedAt) < defaultAuthKeyTTL {
			return entry.value, nil
		}
	}

	// Slow path: regenerate under write lock
	authKeyCache.mu.Lock()
	defer authKeyCache.mu.Unlock()

	// Re-check after acquiring write lock in case another goroutine refreshed it
	if entry2, ok2 := authKeyCache.m[cacheKey]; ok2 {
		if time.Since(entry2.generatedAt) < defaultAuthKeyTTL {
			return entry2.value, nil
		}
	}

	newKey, err := GenerateAuthKey(partnerServiceID, presharedKey)
	if err != nil {
		return "", err
	}
	authKeyCache.m[cacheKey] = cachedAuthKey{value: newKey, generatedAt: time.Now()}
	return newKey, nil
}
