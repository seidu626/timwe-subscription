package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/common/secretcrypto"
)

// DBChannelCredentialSecretStore implements ChannelCredentialSecretStore against
// the tenant_channel_secrets Postgres table using AES-256-GCM encryption.
type DBChannelCredentialSecretStore struct {
	db *sql.DB
}

// NewDBChannelCredentialSecretStore returns a store backed by db.
// The store reads its encryption key from TENANT_SECRET_MASTER_KEY at call time;
// it fails closed when the env var is missing or malformed.
func NewDBChannelCredentialSecretStore(db *sql.DB) *DBChannelCredentialSecretStore {
	return &DBChannelCredentialSecretStore{db: db}
}

// PutChannelCredential encrypts input.SecretValue and stores it, returning a
// secret:// ref and a fingerprint input for the caller to hash.
func (s *DBChannelCredentialSecretStore) PutChannelCredential(ctx context.Context, input ChannelCredentialSecretInput) (ChannelCredentialSecretRef, error) {
	if input.SecretValue == "" {
		return ChannelCredentialSecretRef{}, fmt.Errorf("%w: secret_value is required", ErrAdminDependencyUnavailable)
	}

	// Validate that SecretValue is parseable JSON so we can round-trip it later.
	if !json.Valid([]byte(input.SecretValue)) {
		return ChannelCredentialSecretRef{}, fmt.Errorf("%w: secret_value must be valid JSON", ErrAdminDependencyUnavailable)
	}

	ciphertext, err := secretcrypto.Encrypt([]byte(input.SecretValue))
	if err != nil {
		return ChannelCredentialSecretRef{}, fmt.Errorf("encrypt secret: %w", err)
	}

	purpose := normalizeCredentialPurpose(input.Purpose)

	id := uuid.NewString()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tenant_channel_secrets
			(id, tenant_id, channel_id, purpose, ciphertext, key_version)
		VALUES
			($1::uuid, $2::uuid, $3::uuid, $4, $5, 1)
	`, id, input.TenantID, input.ChannelID, purpose, ciphertext)
	if err != nil {
		return ChannelCredentialSecretRef{}, fmt.Errorf("store secret: %w", err)
	}

	secretRef := "secret://" + id
	return ChannelCredentialSecretRef{
		SecretRef:        secretRef,
		SecretRefDisplay: secretRef, // redacted by caller
		FingerprintInput: input.SecretValue,
	}, nil
}

// GetChannelCredentialSecret looks up and decrypts the secret for the given id (UUID).
// Returns the plaintext JSON blob.
func GetChannelCredentialSecret(ctx context.Context, db *sql.DB, id string) ([]byte, error) {
	var ciphertext []byte
	err := db.QueryRowContext(ctx, `
		SELECT ciphertext FROM tenant_channel_secrets WHERE id = $1::uuid
	`, id).Scan(&ciphertext)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("secret not found: %s", id)
		}
		return nil, fmt.Errorf("fetch secret: %w", err)
	}
	return secretcrypto.Decrypt(ciphertext)
}
