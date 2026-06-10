package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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

// channelProviderSecret mirrors the JSON blob stored for a channel credential.
// Only the fields needed by acquisition-api are decoded here; the rest are ignored.
type channelProviderSecret struct {
	MCC          string `json:"mcc"`
	MNC          string `json:"mnc"`
	LargeAccount string `json:"large_account"`
}

// ChannelAccountConfig holds per-tenant account fields extracted from the channel
// credential blob. Empty string means "not set by tenant"; callers fall back to
// the global config values.
type ChannelAccountConfig struct {
	MCC          string
	MNC          string
	LargeAccount string
}

// GetChannelAccountConfig resolves the active provider_api credential for
// (tenantID, channelID), decrypts it, and returns MCC/MNC/LargeAccount.
// Returns a zero-value ChannelAccountConfig (all empty) when no active credential
// exists — callers must apply their own global-config fallback.
func GetChannelAccountConfig(ctx context.Context, db *sql.DB, tenantID, channelID string) (ChannelAccountConfig, error) {
	var secretRef string
	err := db.QueryRowContext(ctx, `
		SELECT cred.secret_ref
		FROM tenant_channel_credentials cred
		WHERE cred.tenant_id = $1::uuid
		  AND cred.channel_id = $2::uuid
		  AND cred.purpose = 'provider_api'
		  AND cred.status = 'ACTIVE'
		LIMIT 1
	`, tenantID, channelID).Scan(&secretRef)
	if err != nil {
		if err == sql.ErrNoRows {
			return ChannelAccountConfig{}, nil
		}
		return ChannelAccountConfig{}, fmt.Errorf("fetch channel credential: %w", err)
	}

	const secretScheme = "secret://"
	if !strings.HasPrefix(secretRef, secretScheme) {
		// Not a locally-stored secret (e.g. vault://, env://) — no blob to decode.
		return ChannelAccountConfig{}, nil
	}
	secretID := strings.TrimPrefix(secretRef, secretScheme)

	plaintext, err := GetChannelCredentialSecret(ctx, db, secretID)
	if err != nil {
		return ChannelAccountConfig{}, fmt.Errorf("decrypt channel credential: %w", err)
	}

	var s channelProviderSecret
	if err := json.Unmarshal(plaintext, &s); err != nil {
		return ChannelAccountConfig{}, fmt.Errorf("parse channel credential blob: %w", err)
	}
	return ChannelAccountConfig{
		MCC:          strings.TrimSpace(s.MCC),
		MNC:          strings.TrimSpace(s.MNC),
		LargeAccount: strings.TrimSpace(s.LargeAccount),
	}, nil
}
