package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// setTestMasterKey registers a random 32-byte AES key in the test environment.
func setTestMasterKey(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	t.Setenv("TENANT_SECRET_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
}

func TestPutChannelCredential_ReturnsSecretRef(t *testing.T) {
	setTestMasterKey(t)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO tenant_channel_secrets")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewDBChannelCredentialSecretStore(db)
	ref, err := store.PutChannelCredential(context.Background(), ChannelCredentialSecretInput{
		TenantID:    "aaaaaaaa-0000-0000-0000-000000000001",
		ChannelID:   "bbbbbbbb-0000-0000-0000-000000000002",
		Purpose:     "provider_api",
		SecretValue: `{"base_url":"https://api.example.com","api_key":"k1","psk":"p1","partner_role_id":"42"}`,
	})
	if err != nil {
		t.Fatalf("PutChannelCredential: %v", err)
	}
	const prefix = "secret://"
	if len(ref.SecretRef) <= len(prefix) || ref.SecretRef[:len(prefix)] != prefix {
		t.Errorf("expected secret:// ref, got %q", ref.SecretRef)
	}
	if ref.FingerprintInput == "" {
		t.Error("expected non-empty fingerprint input")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations: %v", err)
	}
}

func TestPutChannelCredential_RejectsNonJSON(t *testing.T) {
	setTestMasterKey(t)

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDBChannelCredentialSecretStore(db)
	_, err = store.PutChannelCredential(context.Background(), ChannelCredentialSecretInput{
		TenantID:    "aaaaaaaa-0000-0000-0000-000000000001",
		ChannelID:   "bbbbbbbb-0000-0000-0000-000000000002",
		Purpose:     "provider_api",
		SecretValue: "not-json",
	})
	if err == nil {
		t.Fatal("expected error for non-JSON secret value, got nil")
	}
}

func TestPutChannelCredential_FailsClosedWithoutMasterKey(t *testing.T) {
	t.Setenv("TENANT_SECRET_MASTER_KEY", "")

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDBChannelCredentialSecretStore(db)
	_, err = store.PutChannelCredential(context.Background(), ChannelCredentialSecretInput{
		TenantID:    "aaaaaaaa-0000-0000-0000-000000000001",
		ChannelID:   "bbbbbbbb-0000-0000-0000-000000000002",
		Purpose:     "provider_api",
		SecretValue: `{"api_key":"k1"}`,
	})
	if err == nil {
		t.Fatal("expected error when master key is missing, got nil")
	}
}

func TestPutChannelCredential_EmptySecretValue(t *testing.T) {
	setTestMasterKey(t)

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDBChannelCredentialSecretStore(db)
	_, err = store.PutChannelCredential(context.Background(), ChannelCredentialSecretInput{
		TenantID:    "aaaaaaaa-0000-0000-0000-000000000001",
		ChannelID:   "bbbbbbbb-0000-0000-0000-000000000002",
		Purpose:     "provider_api",
		SecretValue: "",
	})
	if err == nil {
		t.Fatal("expected error for empty secret value, got nil")
	}
}
