package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"go.uber.org/zap"
)

const subscriptionExternalTestDatabaseURL = "TEST_SUBSCRIPTION_EXTERNAL_DATABASE_URL"

func TestUpsertBlacklistedUserUsesTenantConflictKey(t *testing.T) {
	db := openIsolatedRepositoryTestDB(t)
	repo := NewUserBaseRepository(db, zap.NewNop(), nil)
	ctx := context.Background()

	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id) VALUES ($1), ($2)`, tenantA, tenantB); err != nil {
		t.Fatalf("seed tenants: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO userbase (tenant_id, msisdn, type)
		VALUES ($1, 'shared-test-user', 'Staff'), ($2, 'shared-test-user', 'Regular')
	`, tenantA, tenantB); err != nil {
		t.Fatalf("seed tenant-scoped userbase rows: %v", err)
	}

	if err := repo.UpsertBlacklistedUser(ctx, tenantA, "shared-test-user"); err != nil {
		t.Fatalf("upsert blacklist row: %v", err)
	}
	if err := repo.UpsertBlacklistedUser(ctx, tenantA, "shared-test-user"); err != nil {
		t.Fatalf("repeat blacklist upsert: %v", err)
	}

	var tenantAType, tenantBType string
	if err := db.QueryRowContext(ctx, `SELECT type FROM userbase WHERE tenant_id = $1 AND msisdn = 'shared-test-user'`, tenantA).Scan(&tenantAType); err != nil {
		t.Fatalf("read tenant A row: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT type FROM userbase WHERE tenant_id = $1 AND msisdn = 'shared-test-user'`, tenantB).Scan(&tenantBType); err != nil {
		t.Fatalf("read tenant B row: %v", err)
	}
	if tenantAType != "BLACKLISTED" {
		t.Fatalf("tenant A type = %q, want BLACKLISTED", tenantAType)
	}
	if tenantBType != "Regular" {
		t.Fatalf("tenant B type = %q, want Regular", tenantBType)
	}

	var tenantARows int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM userbase WHERE tenant_id = $1 AND msisdn = 'shared-test-user'`, tenantA).Scan(&tenantARows); err != nil {
		t.Fatalf("count tenant A rows: %v", err)
	}
	if tenantARows != 1 {
		t.Fatalf("tenant A rows = %d, want 1", tenantARows)
	}

	excludedA, err := repo.IsExcludedUserForTenant(ctx, tenantA, "shared-test-user")
	if err != nil {
		t.Fatalf("check tenant A exclusion: %v", err)
	}
	excludedB, err := repo.IsExcludedUserForTenant(ctx, tenantB, "shared-test-user")
	if err != nil {
		t.Fatalf("check tenant B exclusion: %v", err)
	}
	if !excludedA {
		t.Fatal("tenant A blacklist was not detected")
	}
	if excludedB {
		t.Fatal("tenant A blacklist leaked into tenant B")
	}
}

func TestUpsertBlacklistedUserRejectsMissingScope(t *testing.T) {
	repo := &UserBaseRepository{}
	if err := repo.UpsertBlacklistedUser(context.Background(), "", "test-user"); err == nil || !strings.Contains(err.Error(), "tenant_id is required") {
		t.Fatalf("missing tenant error = %v", err)
	}
	if err := repo.UpsertBlacklistedUser(context.Background(), uuid.NewString(), ""); err == nil || !strings.Contains(err.Error(), "msisdn is required") {
		t.Fatalf("missing msisdn error = %v", err)
	}
	if _, err := repo.IsExcludedUserForTenant(context.Background(), "", "test-user"); err == nil || !strings.Contains(err.Error(), "tenant_id is required") {
		t.Fatalf("missing exclusion tenant error = %v", err)
	}
}

func TestIsExcludedUserForTenantIgnoresStaleNegativeWhenPositiveCacheWriteFails(t *testing.T) {
	db := openIsolatedRepositoryTestDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	msisdn := "stale-negative-test-user"
	cache := &staleNegativeRedis{
		value:  "Non-Excluded",
		setErr: errors.New("cache replacement failed"),
	}
	repo := NewUserBaseRepository(db, zap.NewNop(), cache)

	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO userbase (tenant_id, msisdn, type)
		VALUES ($1, $2, 'BLACKLISTED')
	`, tenantID, msisdn); err != nil {
		t.Fatalf("commit blacklist row: %v", err)
	}

	excluded, err := repo.IsExcludedUserForTenant(ctx, tenantID, msisdn)
	if err != nil {
		t.Fatalf("check tenant exclusion: %v", err)
	}
	if !excluded {
		t.Fatal("committed blacklist was hidden by stale Non-Excluded cache entry")
	}
	if cache.setCalls != 1 {
		t.Fatalf("positive cache replacement attempts = %d, want 1", cache.setCalls)
	}
	if cache.lastSetValue != "BLACKLISTED" {
		t.Fatalf("positive cache replacement value = %q, want BLACKLISTED", cache.lastSetValue)
	}
}

func TestLiveCompatibleUserbaseSchemaRejectsLegacyGlobalConflictTarget(t *testing.T) {
	db := openIsolatedRepositoryTestDB(t)
	tenantID := uuid.NewString()
	if _, err := db.Exec(`INSERT INTO tenants (id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	_, err := db.Exec(`
		INSERT INTO userbase (tenant_id, msisdn, type)
		VALUES ($1, 'legacy-conflict-user', 'BLACKLISTED')
		ON CONFLICT (msisdn) DO UPDATE SET type = EXCLUDED.type
	`, tenantID)
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || string(pqErr.Code) != "42P10" {
		t.Fatalf("legacy conflict target error = %v, want PostgreSQL 42P10", err)
	}
}

func openIsolatedRepositoryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(subscriptionExternalTestDatabaseURL))
	if dsn == "" {
		t.Skip(subscriptionExternalTestDatabaseURL + " is not set; PostgreSQL repository behavior not verified")
	}

	baseDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = baseDB.Close() })
	if err := baseDB.Ping(); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	schema := "blacklist_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := baseDB.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema)); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := baseDB.Exec(`DROP SCHEMA ` + pq.QuoteIdentifier(schema) + ` CASCADE`); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})

	scopedDSN, err := repositoryTestDSNWithSearchPath(dsn, schema)
	if err != nil {
		t.Fatalf("set test search_path: %v", err)
	}
	db, err := sql.Open("postgres", scopedDSN)
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping scoped PostgreSQL: %v", err)
	}

	const schemaSQL = `
		CREATE TABLE tenants (
			id UUID PRIMARY KEY
		);
		CREATE TABLE userbase (
			id SERIAL PRIMARY KEY,
			msisdn VARCHAR(32) NOT NULL,
			type VARCHAR(32) NOT NULL,
			tenant_id UUID NOT NULL REFERENCES tenants(id)
		);
		CREATE INDEX idx_userbase_msisdn ON userbase (msisdn);
		CREATE INDEX idx_userbase_type ON userbase (type);
		CREATE UNIQUE INDEX idx_userbase_tenant_msisdn ON userbase (tenant_id, msisdn);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("create live-compatible userbase schema: %v", err)
	}

	return db
}

func repositoryTestDSNWithSearchPath(dsn, schema string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse database URL: %w", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type staleNegativeRedis struct {
	value        string
	setErr       error
	setCalls     int
	lastSetValue string
}

func (c *staleNegativeRedis) Get(context.Context, string) (string, error) {
	return c.value, nil
}

func (c *staleNegativeRedis) Set(_ context.Context, _ string, value interface{}, _ time.Duration) error {
	c.setCalls++
	c.lastSetValue = fmt.Sprint(value)
	return c.setErr
}

func (c *staleNegativeRedis) MGet(context.Context, ...string) ([]interface{}, error) {
	return nil, nil
}

func (c *staleNegativeRedis) Exists(context.Context, ...string) (int64, error) {
	return 0, nil
}

func (c *staleNegativeRedis) SetNX(context.Context, string, interface{}, time.Duration) (bool, error) {
	return false, nil
}

func (c *staleNegativeRedis) Del(context.Context, ...string) (int64, error) {
	return 0, nil
}

func (c *staleNegativeRedis) Expire(context.Context, string, time.Duration) (bool, error) {
	return false, nil
}

func (c *staleNegativeRedis) Keys(context.Context, string) ([]string, error) {
	return nil, nil
}

func (c *staleNegativeRedis) Scan(context.Context, uint64, string, int64) ([]string, uint64, error) {
	return nil, 0, nil
}

func (c *staleNegativeRedis) HSet(context.Context, string, ...interface{}) (int64, error) {
	return 0, nil
}

func (c *staleNegativeRedis) HGetAll(context.Context, string) (map[string]string, error) {
	return nil, nil
}

func (c *staleNegativeRedis) Ping(context.Context) (string, error) {
	return "PONG", nil
}

func (c *staleNegativeRedis) Mode() cached.RedisMode {
	return cached.RedisModeFallback
}

func (c *staleNegativeRedis) Close() error {
	return nil
}
