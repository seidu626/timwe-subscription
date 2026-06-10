package secretcrypto_test

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"

	"github.com/seidu626/subscription-manager/common/secretcrypto"
)

func newTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Setenv("TENANT_SECRET_MASTER_KEY", newTestKey(t))

	plaintext := []byte(`{"base_url":"https://api.example.com","api_key":"k1","psk":"p1"}`)
	blob, err := secretcrypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := secretcrypto.Decrypt(blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("round-trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	t.Setenv("TENANT_SECRET_MASTER_KEY", newTestKey(t))
	blob, err := secretcrypto.Encrypt([]byte("secret payload"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Switch to a different key — decrypt must fail.
	t.Setenv("TENANT_SECRET_MASTER_KEY", newTestKey(t))
	_, err = secretcrypto.Decrypt(blob)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key, got nil")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	t.Setenv("TENANT_SECRET_MASTER_KEY", newTestKey(t))
	blob, err := secretcrypto.Encrypt([]byte("tamper test"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flip the last byte.
	tampered := make([]byte, len(blob))
	copy(tampered, blob)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = secretcrypto.Decrypt(tampered)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext, got nil")
	}
}

func TestMissingMasterKey(t *testing.T) {
	os.Unsetenv("TENANT_SECRET_MASTER_KEY")
	_, err := secretcrypto.Encrypt([]byte("hello"))
	if err == nil {
		t.Fatal("expected error when master key is missing")
	}
}

func TestInvalidBase64MasterKey(t *testing.T) {
	t.Setenv("TENANT_SECRET_MASTER_KEY", "not-valid-base64!!!")
	_, err := secretcrypto.Encrypt([]byte("hello"))
	if err == nil {
		t.Fatal("expected error for invalid base64 key")
	}
}

func TestShortMasterKey(t *testing.T) {
	// 16 bytes — valid base64 but wrong length.
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	t.Setenv("TENANT_SECRET_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	_, err := secretcrypto.Encrypt([]byte("hello"))
	if err == nil {
		t.Fatal("expected error for 16-byte (not 32-byte) master key")
	}
}
