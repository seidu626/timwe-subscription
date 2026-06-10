package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seidu626/subscription-manager/common/secretcrypto"
)

// DBProviderCredentialResolver resolves "secret://<uuid>" refs by looking up
// the tenant_channel_secrets table and decrypting the AES-256-GCM ciphertext.
type DBProviderCredentialResolver struct {
	db *sql.DB
}

// NewDBProviderCredentialResolver returns a resolver backed by db.
func NewDBProviderCredentialResolver(db *sql.DB) *DBProviderCredentialResolver {
	return &DBProviderCredentialResolver{db: db}
}

func (r *DBProviderCredentialResolver) ResolveProviderCredential(ctx context.Context, secretRef string) (ProviderCredentialSecret, error) {
	const prefix = "secret://"
	if !strings.HasPrefix(secretRef, prefix) {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: unsupported secret reference scheme for db resolver", ErrTenantCredentialInvalid)
	}
	id := strings.TrimSpace(strings.TrimPrefix(secretRef, prefix))
	if id == "" {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: empty secret id in ref", ErrTenantCredentialInvalid)
	}

	var ciphertext []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT ciphertext FROM tenant_channel_secrets WHERE id = $1::uuid
	`, id).Scan(&ciphertext)
	if err != nil {
		if err == sql.ErrNoRows {
			return ProviderCredentialSecret{}, fmt.Errorf("%w: secret not found: %s", ErrTenantCredentialMissing, id)
		}
		return ProviderCredentialSecret{}, fmt.Errorf("fetch secret: %w", err)
	}

	plaintext, err := secretcrypto.Decrypt(ciphertext)
	if err != nil {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: decrypt: %w", ErrTenantCredentialInvalid, err)
	}

	var secret ProviderCredentialSecret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: invalid credential json: %w", ErrTenantCredentialInvalid, err)
	}
	return secret, nil
}

// CompositeProviderCredentialResolver dispatches to sub-resolvers by URI scheme.
// Unknown schemes return ErrTenantCredentialInvalid.
type CompositeProviderCredentialResolver struct {
	resolvers map[string]ProviderCredentialResolver
}

// NewCompositeProviderCredentialResolver builds a composite resolver from the
// provided scheme → resolver map (e.g. "env" → EnvProviderCredentialResolver{},
// "secret" → DBProviderCredentialResolver).
func NewCompositeProviderCredentialResolver(resolvers map[string]ProviderCredentialResolver) *CompositeProviderCredentialResolver {
	return &CompositeProviderCredentialResolver{resolvers: resolvers}
}

func (c *CompositeProviderCredentialResolver) ResolveProviderCredential(ctx context.Context, secretRef string) (ProviderCredentialSecret, error) {
	scheme := uriScheme(secretRef)
	r, ok := c.resolvers[scheme]
	if !ok {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: no resolver for scheme %q", ErrTenantCredentialInvalid, scheme)
	}
	return r.ResolveProviderCredential(ctx, secretRef)
}

// uriScheme extracts the scheme from a URI like "secret://…" → "secret".
func uriScheme(ref string) string {
	if idx := strings.Index(ref, "://"); idx > 0 {
		return ref[:idx]
	}
	return ""
}
