package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// PKCS7 padding implementation
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// PKCS5 padding function
func pkcs5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// GenerateAuthKeyV2 creates a Base64-encoded AES-ECB token of the form:
//
//	partnerServiceID#timestamp
//
// It mirrors the Python logic: pads with PKCS7, encrypts with AES-ECB,
// and returns the Base64 string.
func GenerateAuthKeyV2(partnerServiceID, presharedKey string) (string, error) {
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

func EncryptPhraseGCM(presharedKey string, partnerServiceId string) (string, error) {
	phraseToEncrypt := fmt.Sprintf("%s#%d", partnerServiceId, time.Now().UnixMilli())

	block, err := aes.NewCipher([]byte(presharedKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	encrypted := gcm.Seal(nonce, nonce, []byte(phraseToEncrypt), nil)

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func EncryptPhrase(presharedKey string, partnerServiceId string) (string, error) {
	// Create the phrase to encrypt with current timestamp in milliseconds
	phraseToEncrypt := fmt.Sprintf("%s#%d", partnerServiceId, time.Now().UnixMilli())

	// Create AES cipher block
	block, err := aes.NewCipher([]byte(presharedKey))
	if err != nil {
		return "", err
	}

	// Add PKCS5 padding
	plaintext := pkcs5Padding([]byte(phraseToEncrypt), aes.BlockSize)

	// Create buffer for encrypted data
	encrypted := make([]byte, len(plaintext))

	// ECB mode encryption (encrypting block by block)
	for i := 0; i < len(plaintext); i += aes.BlockSize {
		block.Encrypt(encrypted[i:i+aes.BlockSize], plaintext[i:i+aes.BlockSize])
	}

	// Encode to base64
	return base64.StdEncoding.EncodeToString(encrypted), nil
}
func GenerateAuthKeyExplicit(partnerServiceID string, presharedKey string) (string, error) {
	timestamp := time.Now().UnixMilli()
	token := fmt.Sprintf("%s#%d", partnerServiceID, timestamp)

	keyBytes := []byte(presharedKey)
	if len(keyBytes) != 16 && len(keyBytes) != 24 && len(keyBytes) != 32 {
		return "", fmt.Errorf("invalid key length: %d", len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	tokenBytes := []byte(token)

	// PKCS7 padding
	blockSize := block.BlockSize()
	padding := blockSize - len(tokenBytes)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	paddedToken := append(tokenBytes, padText...)

	// ECB encryption
	encrypted := make([]byte, len(paddedToken))
	for start := 0; start < len(paddedToken); start += blockSize {
		block.Encrypt(encrypted[start:start+blockSize], paddedToken[start:start+blockSize])
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func GenerateAuthKey(partnerServiceID string, presharedKey string) (string, error) {
	// Get current timestamp in milliseconds
	timestamp := time.Now().UnixMilli()

	// Create the token format: partnerServiceId#timestamp
	token := fmt.Sprintf("%s#%d", partnerServiceID, timestamp)

	// Convert to bytes
	tokenBytes := []byte(token)

	// Ensure key is proper length (16, 24, or 32 bytes for AES)
	keyBytes := []byte(presharedKey)

	// Create cipher in ECB mode
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	// Pad the token
	paddedToken := pkcs7Pad(tokenBytes, aes.BlockSize)

	// Encrypt (ECB mode - encrypt each block independently)
	encrypted := make([]byte, len(paddedToken))
	for i := 0; i < len(paddedToken); i += aes.BlockSize {
		block.Encrypt(encrypted[i:i+aes.BlockSize], paddedToken[i:i+aes.BlockSize])
	}

	// Convert to base64
	return base64.StdEncoding.EncodeToString(encrypted), nil
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

	newKey, err := GenerateAuthKeyV2(partnerServiceID, presharedKey)
	if err != nil {
		return "", err
	}
	authKeyCache.m[cacheKey] = cachedAuthKey{value: newKey, generatedAt: time.Now()}
	return newKey, nil
}
