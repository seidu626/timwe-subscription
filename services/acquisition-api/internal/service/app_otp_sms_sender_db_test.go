package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// TestTenantSMSSenderAgainstPostgres exercises the REAL credential
// resolution SQL (migrations applied verbatim, careerify seed, env://
// secret ref) against a live Postgres, closing the gap left by the
// injected-resolver unit tests. Skipped unless TEST_DATABASE_URL is set:
//
//	TEST_DATABASE_URL=postgres://user:pass@127.0.0.1:5499/otp_it?sslmode=disable go test ...
func TestTenantSMSSenderAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// products is owned by GORM auto-migration in production; the admin
	// migration only ALTERs it, so a minimal stub satisfies the reference.
	for _, stub := range []string{
		`CREATE TABLE IF NOT EXISTS products (id SERIAL PRIMARY KEY, product_id VARCHAR(64) UNIQUE)`,
		`CREATE TABLE IF NOT EXISTS userbase (id SERIAL PRIMARY KEY, msisdn VARCHAR(32))`,
	} {
		if _, err := db.Exec(stub); err != nil {
			t.Fatalf("stub base table: %v", err)
		}
	}

	for _, f := range []string{
		"add_admin_management_tables.sql",
		"add_tenant_channels.sql",
		"add_tenant_channel_credentials.sql",
		"add_tenant_channel_secrets.sql",
		"seed_careerify_tenant_channel.sql",
		"add_tenant_sms_api_unique_active.sql",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", f))
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("apply migration %s: %v", f, err)
		}
	}

	var gotTo, gotSMS string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTo = r.URL.Query().Get("to")
		gotSMS = r.URL.Query().Get("sms")
		fmt.Fprint(w, `{"code":"ok","balance":9}`)
	}))
	defer gateway.Close()

	blob, _ := json.Marshal(map[string]string{
		"url":                   gateway.URL + "/sms/api?action=send-sms&api_key=test-key&to={{msisdn}}&from={{sender}}&sms={{text}}",
		"sender_id":             "Dayline",
		"success_body_contains": `"code":"ok"`,
	})
	t.Setenv("TEST_SMS_GATEWAY_CONFIG", string(blob))

	fingerprint := strings.Repeat("ab", 32)
	insertCred := `
		INSERT INTO tenant_channel_credentials
			(id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display, secret_fingerprint)
		SELECT gen_random_uuid(), t.id, c.id, 'sms_api', $1, $2, 'env://TEST_SMS_GATEWAY_CONFIG', 'env://TEST_SMS_GATEWAY_CONFIG', $3
		FROM tenants t
		JOIN tenant_channels c ON c.tenant_id = t.id
		WHERE t.tenant_key = 'careerify'
		LIMIT 1`
	if _, err := db.Exec(insertCred, 1, "ACTIVE", fingerprint); err != nil {
		t.Fatalf("seed sms_api credential: %v", err)
	}

	// The reviewer-driven partial unique index must reject a second ACTIVE
	// sms_api credential for the same tenant.
	if _, err := db.Exec(insertCred, 2, "ACTIVE", fingerprint); err == nil {
		t.Fatal("second ACTIVE sms_api credential should violate uniq_tenant_channel_credentials_sms_api_active")
	}

	sender := NewTenantSMSSender(db, zap.NewNop())
	if err := sender.SendLoginOTP("233241234567", "careerify", "654321"); err != nil {
		t.Fatalf("SendLoginOTP through real resolution path: %v", err)
	}
	if gotTo != "233241234567" {
		t.Errorf("gateway to = %q", gotTo)
	}
	if !strings.Contains(gotSMS, "654321") {
		t.Errorf("gateway sms %q missing code", gotSMS)
	}

	// Unknown tenant fails closed.
	if err := sender.SendLoginOTP("233241234567", "no-such-tenant", "654321"); err == nil {
		t.Error("unknown tenant should fail closed")
	}
}
