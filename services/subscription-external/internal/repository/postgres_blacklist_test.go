package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

func TestDeleteSubscriptionRecordsForTenantDoesNotCrossTenantBoundary(t *testing.T) {
	db := openIsolatedRepositoryTestDB(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id) VALUES ($1), ($2)`, tenantA, tenantB); err != nil {
		t.Fatalf("seed tenants: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE subscriptions (
			id BIGSERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL REFERENCES tenants(id),
			user_identifier VARCHAR(32) NOT NULL
		)
	`); err != nil {
		t.Fatalf("create subscriptions table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO subscriptions (tenant_id, user_identifier)
		VALUES ($1, 'shared-test-user'), ($2, 'shared-test-user')
	`, tenantA, tenantB); err != nil {
		t.Fatalf("seed subscriptions: %v", err)
	}

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if err := repo.DeleteSubscriptionRecordsForTenant(ctx, tenantA, "shared-test-user"); err != nil {
		t.Fatalf("delete tenant subscriptions: %v", err)
	}

	var rowsA, rowsB int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM subscriptions WHERE tenant_id = $1`, tenantA).Scan(&rowsA); err != nil {
		t.Fatalf("count tenant A subscriptions: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM subscriptions WHERE tenant_id = $1`, tenantB).Scan(&rowsB); err != nil {
		t.Fatalf("count tenant B subscriptions: %v", err)
	}
	if rowsA != 0 || rowsB != 1 {
		t.Fatalf("subscription counts after tenant A delete = (%d, %d), want (0, 1)", rowsA, rowsB)
	}
}

func TestDeleteSubscriptionRecordsForTenantRejectsMissingScope(t *testing.T) {
	repo := &SubscriptionRepository{}
	if err := repo.DeleteSubscriptionRecordsForTenant(context.Background(), "", "test-user"); err == nil || !strings.Contains(err.Error(), "tenant_id is required") {
		t.Fatalf("missing tenant error = %v", err)
	}
	if err := repo.DeleteSubscriptionRecordsForTenant(context.Background(), uuid.NewString(), ""); err == nil || !strings.Contains(err.Error(), "msisdn is required") {
		t.Fatalf("missing msisdn error = %v", err)
	}
}

func TestCreateSubscriptionHonorsCancelledRepositoryContext(t *testing.T) {
	tenantID := uuid.NewString()
	tests := []struct {
		name     string
		tenantID *string
	}{
		{name: "tenant scoped", tenantID: &tenantID},
		{name: "legacy", tenantID: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Fatalf("create mock database: %v", err)
			}
			defer db.Close()

			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			repo := &SubscriptionRepository{db: db, logger: zap.NewNop(), ctx: cancelled}
			err = repo.CreateSubscription(&domain.SubscriptionRequest{
				TenantID:       tt.tenantID,
				UserIdentifier: "test-user",
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("CreateSubscription error = %v, want context cancellation", err)
			}
		})
	}
}
