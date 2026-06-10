// Package service — go-sqlmock tests that pin the SQL shape of the scoped
// tenant-credential resolver JOIN.  These tests are always-on (no Docker
// required) and will FAIL if the JOIN clause is reverted to the pre-23bce0d
// form (`cred.secret_ref = $1` instead of
// `cred.secret_ref = 'secret://' || s.id::text`).
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/common/secretcrypto"
)

func setMasterKeyForSQLMock(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	t.Setenv("TENANT_SECRET_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
}

func encryptCredForSQLMock(t *testing.T, cred ProviderCredentialSecret) []byte {
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

// scopedJOINPattern must match the SQL emitted by ResolveProviderCredential
// when tenantID+channelID are non-empty.  It asserts THREE invariants from
// commit 23bce0d:
//  1. The JOIN condition uses the PREFIXED form:
//     ON cred.secret_ref = 'secret://' || s.id::text
//     (not the bare-UUID form `cred.secret_ref = $1` that caused the regression)
//  2. The tenant and channel are scoped via cred.tenant_id::text = $2 / $3
//  3. The WHERE clause resolves the UUID: WHERE s.id = $1::uuid
//
// The pattern uses regex escaping so special characters in the SQL are matched
// literally (e.g. || is not treated as alternation).
var scopedJOINPattern = regexp.MustCompile(
	`(?s)` + // DOTALL — newlines count
		`FROM\s+tenant_channel_secrets\s+s` +
		`.*JOIN\s+tenant_channel_credentials\s+cred` +
		`.*ON\s+cred\.secret_ref\s*=\s*'secret://'` + // PREFIXED join — the fixed form
		`.*\|\|\s*s\.id::text` + // || s.id::text concatenation
		`.*cred\.tenant_id::text\s*=\s*\$2` + // tenant scope
		`.*cred\.channel_id::text\s*=\s*\$3` + // channel scope
		`.*WHERE\s+s\.id\s*=\s*\$1::uuid`, // row filter
)

// buggyJOINPattern matches the PRE-FIX form (the bug): `cred.secret_ref = $1`.
var buggyJOINPattern = regexp.MustCompile(
	`(?s)cred\.secret_ref\s*=\s*\$1`,
)

// capturingSQLMatcher is a sqlmock QueryMatcher that captures each SQL
// string so tests can assert on the exact query shape.
type capturingSQLMatcher struct {
	captured []string
}

func (m *capturingSQLMatcher) Match(expectedSQL, actualSQL string) error {
	m.captured = append(m.captured, actualSQL)
	return nil // always match — we capture, not gate
}

// TestDBResolver_SQLMock_ScopedJOIN_UsesPrefix verifies that when tenantID and
// channelID are supplied the resolver emits SQL that uses the PREFIXED join form
// fixed by commit 23bce0d.  This test will FAIL if that commit is reverted.
func TestDBResolver_SQLMock_ScopedJOIN_UsesPrefix(t *testing.T) {
	setMasterKeyForSQLMock(t)

	want := ProviderCredentialSecret{
		BaseURL: "https://api.example.com",
		APIKey:  "sqlmock-key",
		Realm:   "GH",
	}
	blob := encryptCredForSQLMock(t, want)

	cm := &capturingSQLMatcher{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(cm))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("").
		WithArgs("some-uuid-1234", "tenant-aaa", "channel-bbb").
		WillReturnRows(mock.NewRows([]string{"ciphertext"}).AddRow(blob))

	resolver := NewDBProviderCredentialResolver(db)
	got, err := resolver.ResolveProviderCredential(
		context.Background(),
		"secret://some-uuid-1234",
		"tenant-aaa",
		"channel-bbb",
	)
	if err != nil {
		t.Fatalf("ResolveProviderCredential: %v", err)
	}
	if got.APIKey != want.APIKey {
		t.Errorf("api_key: got %q want %q", got.APIKey, want.APIKey)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}

	if len(cm.captured) == 0 {
		t.Fatal("no SQL was captured — resolver did not query the DB")
	}
	actualSQL := cm.captured[0]

	// 1. Must match the FIXED (prefixed) JOIN shape.
	if !scopedJOINPattern.MatchString(actualSQL) {
		t.Errorf("SQL does not match expected scoped-JOIN pattern.\nPattern: %s\nSQL    : %s",
			scopedJOINPattern.String(), actualSQL)
	}

	// 2. Must NOT match the BUGGY bare-ref join shape.
	if buggyJOINPattern.MatchString(actualSQL) {
		t.Errorf("SQL still uses the PRE-FIX (regression) form `cred.secret_ref = $1`.\n"+
			"This is the bug fixed in commit 23bce0d.\nSQL: %s", actualSQL)
	}
}

// TestDBResolver_SQLMock_ScopedJOIN_CrossTenantReturnsNotFound verifies that a
// no-rows response surfaces as ErrTenantCredentialMissing.
func TestDBResolver_SQLMock_ScopedJOIN_CrossTenantReturnsNotFound(t *testing.T) {
	setMasterKeyForSQLMock(t)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(mock.NewRows([]string{"ciphertext"})) // empty

	resolver := NewDBProviderCredentialResolver(db)
	_, err = resolver.ResolveProviderCredential(
		context.Background(),
		"secret://some-uuid-1234",
		"wrong-tenant",
		"wrong-channel",
	)
	if err == nil {
		t.Fatal("expected ErrTenantCredentialMissing, got nil")
	}
	if !errors.Is(err, ErrTenantCredentialMissing) {
		t.Errorf("expected ErrTenantCredentialMissing; got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestDBResolver_SQLMock_UnscopedPath_UsesSimpleSelect verifies that when
// tenantID and channelID are empty the resolver uses the unscoped SELECT (no JOIN).
func TestDBResolver_SQLMock_UnscopedPath_UsesSimpleSelect(t *testing.T) {
	setMasterKeyForSQLMock(t)

	cm := &capturingSQLMatcher{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(cm))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("").
		WithArgs("bare-uuid-9999").
		WillReturnRows(mock.NewRows([]string{"ciphertext"}))

	resolver := NewDBProviderCredentialResolver(db)
	_, err = resolver.ResolveProviderCredential(
		context.Background(),
		"secret://bare-uuid-9999",
		"", "",
	)
	if !errors.Is(err, ErrTenantCredentialMissing) {
		t.Errorf("expected ErrTenantCredentialMissing on empty unscoped DB; got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}

	if len(cm.captured) == 0 {
		t.Fatal("no SQL captured")
	}
	if scopedJOINPattern.MatchString(cm.captured[0]) {
		t.Errorf("unscoped path emitted a JOIN: %s", cm.captured[0])
	}
}

// TestDBResolver_SQLMock_ScopedJOIN_DBError verifies that a genuine DB error
// (not sql.ErrNoRows) is NOT wrapped as ErrTenantCredentialMissing.
func TestDBResolver_SQLMock_ScopedJOIN_DBError(t *testing.T) {
	setMasterKeyForSQLMock(t)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	fakeDBErr := fmt.Errorf("pq: connection reset by peer")
	mock.ExpectQuery("").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(fakeDBErr)

	resolver := NewDBProviderCredentialResolver(db)
	_, err = resolver.ResolveProviderCredential(
		context.Background(),
		"secret://some-uuid-xyz",
		"tenant-t1",
		"channel-c1",
	)
	if err == nil {
		t.Fatal("expected error from DB, got nil")
	}
	if errors.Is(err, ErrTenantCredentialMissing) {
		t.Errorf("DB error should NOT be ErrTenantCredentialMissing; got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
