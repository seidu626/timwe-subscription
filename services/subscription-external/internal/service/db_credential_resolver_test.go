package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/seidu626/subscription-manager/common/config"
	"io"
	"sync"
	"testing"

	"github.com/seidu626/subscription-manager/common/secretcrypto"
)

// ─── minimal fake SQL driver for resolver tests ───────────────────────────────

var registerResolverFakeDriverOnce sync.Once
const resolverFakeDriverName = "fake-resolver"

type resolverFakeDriver struct{ blob []byte }
type resolverFakeConn struct{ blob []byte }
type resolverFakeStmt struct{ blob []byte }
type resolverFakeRows struct {
	blob   []byte
	served bool
}

func (d *resolverFakeDriver) Open(_ string) (driver.Conn, error) {
	return &resolverFakeConn{blob: d.blob}, nil
}
func (c *resolverFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return &resolverFakeStmt{blob: c.blob}, nil
}
func (c *resolverFakeConn) Close() error              { return nil }
func (c *resolverFakeConn) Begin() (driver.Tx, error) { return nil, errors.New("unsupported") }
func (s *resolverFakeStmt) Close() error              { return nil }
func (s *resolverFakeStmt) NumInput() int             { return -1 }
func (s *resolverFakeStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, errors.New("unsupported")
}
func (s *resolverFakeStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return &resolverFakeRows{blob: s.blob}, nil
}
func (r *resolverFakeRows) Columns() []string    { return []string{"ciphertext"} }
func (r *resolverFakeRows) Close() error         { return nil }
func (r *resolverFakeRows) Next(dest []driver.Value) error {
	if r.served {
		return io.EOF
	}
	r.served = true
	dest[0] = r.blob
	return nil
}

// openResolverFakeDB returns a *sql.DB that returns the given ciphertext blob on any query.
func openResolverFakeDB(t *testing.T, blob []byte) *sql.DB {
	t.Helper()
	driverName := resolverFakeDriverName + "-" + t.Name()
	sql.Register(driverName, &resolverFakeDriver{blob: blob})
	db, err := sql.Open(driverName, "fake")
	if err != nil {
		t.Fatalf("openResolverFakeDB: %v", err)
	}
	return db
}

// emptyDB returns a fake DB that returns no rows.
type emptyRowsDriver struct{}
type emptyConn struct{}
type emptyStmt struct{}
type emptyRows struct{}

func (d *emptyRowsDriver) Open(_ string) (driver.Conn, error) { return &emptyConn{}, nil }
func (c *emptyConn) Prepare(_ string) (driver.Stmt, error)    { return &emptyStmt{}, nil }
func (c *emptyConn) Close() error                              { return nil }
func (c *emptyConn) Begin() (driver.Tx, error)                { return nil, errors.New("unsupported") }
func (s *emptyStmt) Close() error                              { return nil }
func (s *emptyStmt) NumInput() int                             { return -1 }
func (s *emptyStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, errors.New("unsupported")
}
func (s *emptyStmt) Query(_ []driver.Value) (driver.Rows, error) { return &emptyRows{}, nil }
func (r *emptyRows) Columns() []string                            { return []string{"ciphertext"} }
func (r *emptyRows) Close() error                                 { return nil }
func (r *emptyRows) Next(_ []driver.Value) error                  { return io.EOF }

func openEmptyFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	driverName := "fake-resolver-empty-" + t.Name()
	sql.Register(driverName, &emptyRowsDriver{})
	db, err := sql.Open(driverName, "fake")
	if err != nil {
		t.Fatalf("openEmptyFakeDB: %v", err)
	}
	return db
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func setResolverTestMasterKey(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	t.Setenv("TENANT_SECRET_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
}

func encryptCredential(t *testing.T, cred ProviderCredentialSecret) []byte {
	t.Helper()
	raw, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("marshal cred: %v", err)
	}
	blob, err := secretcrypto.Encrypt(raw)
	if err != nil {
		t.Fatalf("encrypt cred: %v", err)
	}
	return blob
}

// ─── DBProviderCredentialResolver tests ──────────────────────────────────────

func TestDBProviderCredentialResolver_RoundTrip(t *testing.T) {
	setResolverTestMasterKey(t)

	want := ProviderCredentialSecret{
		BaseURL:           "https://api.example.com",
		APIKey:            "mykey",
		AuthenticationKey: "myauth",
		PartnerServiceID:  "svc42",
		PSK:               "mypsk",
		PartnerRoleID:     "7",
		Realm:             "GH",
	}
	blob := encryptCredential(t, want)
	db := openResolverFakeDB(t, blob)
	defer db.Close()

	resolver := NewDBProviderCredentialResolver(db)
	got, err := resolver.ResolveProviderCredential(context.Background(), "secret://some-uuid", "", "")
	if err != nil {
		t.Fatalf("ResolveProviderCredential: %v", err)
	}
	if got != want {
		t.Errorf("credential mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestDBProviderCredentialResolver_NotFound(t *testing.T) {
	setResolverTestMasterKey(t)
	db := openEmptyFakeDB(t)
	defer db.Close()

	resolver := NewDBProviderCredentialResolver(db)
	_, err := resolver.ResolveProviderCredential(context.Background(), "secret://missing-uuid", "", "")
	if err == nil {
		t.Fatal("expected error for missing secret, got nil")
	}
	if !errors.Is(err, ErrTenantCredentialMissing) {
		t.Errorf("expected ErrTenantCredentialMissing, got %v", err)
	}
}

func TestDBProviderCredentialResolver_UnsupportedScheme(t *testing.T) {
	db := openEmptyFakeDB(t)
	defer db.Close()

	resolver := NewDBProviderCredentialResolver(db)
	_, err := resolver.ResolveProviderCredential(context.Background(), "env://SOME_VAR", "", "")
	if err == nil {
		t.Fatal("expected error for unsupported scheme, got nil")
	}
	if !errors.Is(err, ErrTenantCredentialInvalid) {
		t.Errorf("expected ErrTenantCredentialInvalid, got %v", err)
	}
}

// ─── CompositeProviderCredentialResolver tests ────────────────────────────────

func TestCompositeResolver_DispatchesEnv(t *testing.T) {
	cred := ProviderCredentialSecret{BaseURL: "https://env.example.com", APIKey: "envkey", PartnerRoleID: "1"}
	raw, _ := json.Marshal(cred)
	t.Setenv("TEST_CRED_VAR", string(raw))

	composite := NewCompositeProviderCredentialResolver(map[string]ProviderCredentialResolver{
		"env": EnvProviderCredentialResolver{},
	})
	got, err := composite.ResolveProviderCredential(context.Background(), "env://TEST_CRED_VAR", "", "")
	if err != nil {
		t.Fatalf("composite env dispatch: %v", err)
	}
	if got.APIKey != "envkey" {
		t.Errorf("got api_key %q, want %q", got.APIKey, "envkey")
	}
}

func TestCompositeResolver_DispatchesSecret(t *testing.T) {
	setResolverTestMasterKey(t)

	want := ProviderCredentialSecret{BaseURL: "https://db.example.com", APIKey: "dbkey", PartnerRoleID: "2"}
	blob := encryptCredential(t, want)
	db := openResolverFakeDB(t, blob)
	defer db.Close()

	composite := NewCompositeProviderCredentialResolver(map[string]ProviderCredentialResolver{
		"env":    EnvProviderCredentialResolver{},
		"secret": NewDBProviderCredentialResolver(db),
	})
	got, err := composite.ResolveProviderCredential(context.Background(), "secret://some-uuid", "", "")
	if err != nil {
		t.Fatalf("composite secret dispatch: %v", err)
	}
	if got.APIKey != "dbkey" {
		t.Errorf("got api_key %q, want %q", got.APIKey, "dbkey")
	}
}

func TestCompositeResolver_UnknownScheme(t *testing.T) {
	composite := NewCompositeProviderCredentialResolver(map[string]ProviderCredentialResolver{
		"env": EnvProviderCredentialResolver{},
	})
	_, err := composite.ResolveProviderCredential(context.Background(), "vault://some/path", "", "")
	if err == nil {
		t.Fatal("expected error for unknown scheme, got nil")
	}
	if !errors.Is(err, ErrTenantCredentialInvalid) {
		t.Errorf("expected ErrTenantCredentialInvalid, got %v", err)
	}
}

// ─── FIX 2: tenant-scoped DB resolver ────────────────────────────────────────

// TestDBProviderCredentialResolver_TenantScoped_NoRows verifies that the tenant-
// scoped path (tenantID+channelID supplied) returns ErrTenantCredentialMissing
// when the fake DB returns no rows (simulates a secret that doesn't belong to the
// given tenant/channel).
func TestDBProviderCredentialResolver_TenantScoped_NoRows(t *testing.T) {
	setResolverTestMasterKey(t)
	db := openEmptyFakeDB(t)
	defer db.Close()

	resolver := NewDBProviderCredentialResolver(db)
	_, err := resolver.ResolveProviderCredential(
		context.Background(),
		"secret://some-uuid",
		"tenant-123",
		"channel-456",
	)
	if err == nil {
		t.Fatal("expected error when secret not found for tenant/channel, got nil")
	}
	if !errors.Is(err, ErrTenantCredentialMissing) {
		t.Errorf("expected ErrTenantCredentialMissing, got %v", err)
	}
}

// TestDBProviderCredentialResolver_TenantScoped_RoundTrip verifies that when
// tenant/channel are supplied and the fake DB returns the ciphertext, decryption
// succeeds (the scoped SELECT path round-trips correctly).
func TestDBProviderCredentialResolver_TenantScoped_RoundTrip(t *testing.T) {
	setResolverTestMasterKey(t)

	want := ProviderCredentialSecret{
		BaseURL:           "https://api.example.com",
		APIKey:            "scoped-key",
		AuthenticationKey: "scoped-auth",
		PartnerServiceID:  "svc99",
		PSK:               "psk99",
		PartnerRoleID:     "9",
		Realm:             "GH",
	}
	blob := encryptCredential(t, want)
	db := openResolverFakeDB(t, blob)
	defer db.Close()

	resolver := NewDBProviderCredentialResolver(db)
	got, err := resolver.ResolveProviderCredential(
		context.Background(),
		"secret://some-uuid",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"11111111-2222-3333-4444-555555555555",
	)
	if err != nil {
		t.Fatalf("ResolveProviderCredential with tenant scope: %v", err)
	}
	if got != want {
		t.Errorf("credential mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

// ─── FIX 3: SSRF base_url validation ─────────────────────────────────────────

func TestValidateProviderBaseURL_AcceptsHTTPS(t *testing.T) {
	cases := []string{
		"https://api.timwe.com",
		"https://api.timwe.com/v2",
		"https://sub.domain.example.com:8443/path",
	}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			if err := validateProviderBaseURL(u, nil); err != nil {
				t.Errorf("expected accept for %q, got error: %v", u, err)
			}
		})
	}
}

func TestValidateProviderBaseURL_RejectsHTTPInProduction(t *testing.T) {
	// nil config => not DEVELOPMENT => http must be rejected
	err := validateProviderBaseURL("http://api.timwe.com", nil)
	if err == nil {
		t.Error("expected rejection of http:// in production environment, got nil")
	}
}

func TestValidateProviderBaseURL_AcceptsHTTPInDevelopment(t *testing.T) {
	cfg := &config.Config{}
	cfg.Application.Environment = config.DEVELOPMENT
	if err := validateProviderBaseURL("http://api.timwe.com", cfg); err != nil {
		t.Errorf("expected http:// allowed in DEVELOPMENT, got: %v", err)
	}
}

func TestValidateProviderBaseURL_RejectsPrivateIPs(t *testing.T) {
	privateURLs := []string{
		"https://10.0.0.1/api",
		"https://172.16.1.1/api",
		"https://192.168.1.1/api",
		"https://127.0.0.1/api",
		"https://169.254.169.254/api", // AWS metadata
		"https://[::1]/api",
		"https://[fc00::1]/api",
		"https://[fe80::1]/api",
	}
	for _, u := range privateURLs {
		t.Run(u, func(t *testing.T) {
			err := validateProviderBaseURL(u, nil)
			if err == nil {
				t.Errorf("expected rejection of private IP URL %q, got nil", u)
			}
		})
	}
}

func TestValidateProviderBaseURL_RejectsUserinfo(t *testing.T) {
	err := validateProviderBaseURL("https://user:pass@api.example.com", nil)
	if err == nil {
		t.Error("expected rejection of URL with userinfo, got nil")
	}
}

func TestValidateProviderBaseURL_RejectsUnsupportedScheme(t *testing.T) {
	err := validateProviderBaseURL("ftp://api.example.com", nil)
	if err == nil {
		t.Error("expected rejection of ftp:// scheme, got nil")
	}
}

func TestValidateProviderBaseURL_RejectsEmpty(t *testing.T) {
	err := validateProviderBaseURL("", nil)
	if err == nil {
		t.Error("expected rejection of empty base_url, got nil")
	}
}
