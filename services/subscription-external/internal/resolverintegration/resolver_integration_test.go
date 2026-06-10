// Package resolverintegration provides real-Postgres integration tests for the
// scoped tenant-credential resolver (DBProviderCredentialResolver).
//
// Tests use a single shared postgres container (started once via TestMain) so
// each test function incurs no extra container-start overhead.  The container
// is torn down after all tests run.
//
// Run with the integration build tag:
//
//	go test -tags integration ./internal/resolverintegration/... -v -timeout 120s
//
// Without -tags integration these files are excluded from normal `go test ./...`.
//
//go:build integration
// +build integration

package resolverintegration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	dockertest "github.com/ory/dockertest/v3"
	"github.com/seidu626/subscription-manager/common/secretcrypto"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
)

// minimalSchema sets up only what the resolver needs — no FK dependencies on
// tenants / tenant_channels so the test stays self-contained.
const minimalSchema = `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS tenant_channel_secrets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    channel_id   UUID NOT NULL,
    purpose      VARCHAR(80) NOT NULL DEFAULT 'provider_api',
    ciphertext   BYTEA NOT NULL,
    key_version  INTEGER NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_channel_credentials (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    channel_id          UUID NOT NULL,
    purpose             VARCHAR(80) NOT NULL DEFAULT 'provider_api',
    version             INTEGER NOT NULL DEFAULT 1,
    status              VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    secret_ref          TEXT NOT NULL,
    secret_ref_display  TEXT NOT NULL DEFAULT 'test',
    secret_fingerprint  TEXT NOT NULL DEFAULT repeat('a', 64),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

// package-level shared DB — set up once via TestMain.
var sharedDB *sql.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker unavailable, skipping integration tests: %v\n", err)
		os.Exit(0) // skip gracefully
	}
	pool.MaxWait = 60 * time.Second

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "docker.io/library/postgres",
		Tag:        "15-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=testpass",
			"POSTGRES_USER=testuser",
			"POSTGRES_DB=testdb",
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start postgres container, skipping: %v\n", err)
		os.Exit(0)
	}
	defer func() { _ = pool.Purge(resource) }()
	_ = resource.Expire(120)

	dsn := fmt.Sprintf("host=localhost port=%s user=testuser password=testpass dbname=testdb sslmode=disable",
		resource.GetPort("5432/tcp"))

	err = pool.Retry(func() error {
		db, openErr := sql.Open("postgres", dsn)
		if openErr != nil {
			return openErr
		}
		if pingErr := db.Ping(); pingErr != nil {
			_ = db.Close()
			return pingErr
		}
		sharedDB = db
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres not ready in time: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = sharedDB.Close() }()

	if _, err := sharedDB.Exec(minimalSchema); err != nil {
		fmt.Fprintf(os.Stderr, "apply schema: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func setMasterKey(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	t.Setenv("TENANT_SECRET_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
}

func encryptCred(t *testing.T, cred service.ProviderCredentialSecret) []byte {
	t.Helper()
	raw, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	blob, err := secretcrypto.Encrypt(raw)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return blob
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestIntegration_ScopedJOIN_PrefixedSecretRef is the PRIMARY regression test.
//
// It stores a secret in tenant_channel_secrets, inserts a matching credential
// row in tenant_channel_credentials with the `secret://` prefix, then asserts
// the resolver resolves and decrypts the secret correctly.
//
// This test FAILS if the JOIN reverts to `cred.secret_ref = $1` (bare UUID)
// because that form never matches the stored `secret://<uuid>` value — the
// exact bug fixed in commit 23bce0d.
func TestIntegration_ScopedJOIN_PrefixedSecretRef(t *testing.T) {
	setMasterKey(t)

	tenantID := "aaaaaaaa-0000-0000-0000-aaaaaaaaaaaa"
	channelID := "bbbbbbbb-0000-0000-0000-bbbbbbbbbbbb"

	want := service.ProviderCredentialSecret{
		BaseURL: "https://real-pg.example.com",
		APIKey:  "real-pg-key",
		Realm:   "GH",
	}
	blob := encryptCred(t, want)

	var secretID string
	err := sharedDB.QueryRowContext(context.Background(), `
		INSERT INTO tenant_channel_secrets (tenant_id, channel_id, ciphertext)
		VALUES ($1::uuid, $2::uuid, $3)
		RETURNING id::text
	`, tenantID, channelID, blob).Scan(&secretID)
	if err != nil {
		t.Fatalf("insert secret: %v", err)
	}

	// Credential row uses the PREFIXED form — this is what the fixed JOIN matches.
	secretRef := "secret://" + secretID
	_, err = sharedDB.ExecContext(context.Background(), `
		INSERT INTO tenant_channel_credentials
			(tenant_id, channel_id, secret_ref, secret_ref_display, secret_fingerprint)
		VALUES ($1::uuid, $2::uuid, $3, 'display', repeat('b',64))
	`, tenantID, channelID, secretRef)
	if err != nil {
		t.Fatalf("insert credential: %v", err)
	}

	resolver := service.NewDBProviderCredentialResolver(sharedDB)
	got, err := resolver.ResolveProviderCredential(
		context.Background(), secretRef, tenantID, channelID,
	)
	if err != nil {
		t.Fatalf("ResolveProviderCredential: %v", err)
	}
	if got.APIKey != want.APIKey {
		t.Errorf("api_key: got %q want %q", got.APIKey, want.APIKey)
	}
	if got.BaseURL != want.BaseURL {
		t.Errorf("base_url: got %q want %q", got.BaseURL, want.BaseURL)
	}
}

// TestIntegration_CrossTenantRejection verifies that the scoped JOIN prevents a
// different tenant from reading another tenant's secret (defence-in-depth).
func TestIntegration_CrossTenantRejection(t *testing.T) {
	setMasterKey(t)

	ownerTenantID := "cccccccc-0000-0000-0000-cccccccccccc"
	ownerChannelID := "dddddddd-0000-0000-0000-dddddddddddd"
	attackerTenantID := "eeeeeeee-0000-0000-0000-eeeeeeeeeeee"
	attackerChannelID := "ffffffff-0000-0000-0000-ffffffffffff"

	blob := encryptCred(t, service.ProviderCredentialSecret{APIKey: "owner-only"})

	var secretID string
	err := sharedDB.QueryRowContext(context.Background(), `
		INSERT INTO tenant_channel_secrets (tenant_id, channel_id, ciphertext)
		VALUES ($1::uuid, $2::uuid, $3)
		RETURNING id::text
	`, ownerTenantID, ownerChannelID, blob).Scan(&secretID)
	if err != nil {
		t.Fatalf("insert secret: %v", err)
	}

	secretRef := "secret://" + secretID
	_, err = sharedDB.ExecContext(context.Background(), `
		INSERT INTO tenant_channel_credentials
			(tenant_id, channel_id, secret_ref, secret_ref_display, secret_fingerprint)
		VALUES ($1::uuid, $2::uuid, $3, 'display', repeat('c',64))
	`, ownerTenantID, ownerChannelID, secretRef)
	if err != nil {
		t.Fatalf("insert credential: %v", err)
	}

	resolver := service.NewDBProviderCredentialResolver(sharedDB)
	_, err = resolver.ResolveProviderCredential(
		context.Background(), secretRef, attackerTenantID, attackerChannelID,
	)
	if err == nil {
		t.Fatal("expected ErrTenantCredentialMissing for cross-tenant access, got nil")
	}
	if !errors.Is(err, service.ErrTenantCredentialMissing) {
		t.Errorf("expected ErrTenantCredentialMissing; got: %v", err)
	}
}

// TestIntegration_UnscopedFallback verifies that the unscoped path (no tenant/channel)
// still resolves by direct id lookup.
func TestIntegration_UnscopedFallback(t *testing.T) {
	setMasterKey(t)

	tenantID := "11111111-0000-0000-0000-111111111111"
	channelID := "22222222-0000-0000-0000-222222222222"

	want := service.ProviderCredentialSecret{APIKey: "unscoped-key"}
	blob := encryptCred(t, want)

	var secretID string
	err := sharedDB.QueryRowContext(context.Background(), `
		INSERT INTO tenant_channel_secrets (tenant_id, channel_id, ciphertext)
		VALUES ($1::uuid, $2::uuid, $3)
		RETURNING id::text
	`, tenantID, channelID, blob).Scan(&secretID)
	if err != nil {
		t.Fatalf("insert secret: %v", err)
	}

	resolver := service.NewDBProviderCredentialResolver(sharedDB)
	got, err := resolver.ResolveProviderCredential(
		context.Background(), "secret://"+secretID, "", "",
	)
	if err != nil {
		t.Fatalf("ResolveProviderCredential (unscoped): %v", err)
	}
	if got.APIKey != want.APIKey {
		t.Errorf("api_key: got %q want %q", got.APIKey, want.APIKey)
	}
}
