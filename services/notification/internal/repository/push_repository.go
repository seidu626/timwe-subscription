package repository

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

// PushRepository reads the Dayline app device-registration and
// notification-preference tables (owned and migrated by
// subscription-external; see docs/dayline-app-api-contract.md) over this
// service's own Postgres connection, on the shared database instance.
type PushRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewPushRepository builds a PushRepository over the shared Postgres
// connection used by the notification worker.
func NewPushRepository(db *sql.DB, logger *zap.Logger) *PushRepository {
	return &PushRepository{db: db, logger: logger}
}

// DeviceToken returns the most recently registered FCM token for msisdn,
// scoped to tenantID when it is non-empty. app_devices stores tenant_key
// (the tenant's slug, e.g. "careerify"; see migration 024 and
// docs/dayline-app-api-contract.md), while the outbox job carries the
// tenant's UUID id (domain.OutboxJob.TenantID / message_outbox.tenant_id),
// so tenantID is resolved to a tenant_key via a join on the tenants table.
// Without this scoping, one msisdn holding devices registered under
// different white-label tenants could have a push meant for tenant A
// delivered to a device registered under tenant B.
//
// When tenantID is empty (legacy jobs predating the tenant_id backfill; see
// migration 017), the lookup stays unscoped, preserving current behavior:
// there is no tenant to scope by, and refusing delivery outright would
// silently drop push notifications for those jobs.
//
// found is false when msisdn has no registered (tenant-matching) device
// (not an error).
func (r *PushRepository) DeviceToken(ctx context.Context, msisdn, tenantID string) (string, bool, error) {
	var (
		token string
		err   error
	)
	if tenantID == "" {
		const query = `
			SELECT fcm_token
			FROM app_devices
			WHERE msisdn = $1
			ORDER BY updated_at DESC
			LIMIT 1`
		err = r.db.QueryRowContext(ctx, query, msisdn).Scan(&token)
	} else {
		const query = `
			SELECT ad.fcm_token
			FROM app_devices ad
			JOIN tenants t ON t.tenant_key = ad.tenant_key
			WHERE ad.msisdn = $1 AND t.id = $2::uuid
			ORDER BY ad.updated_at DESC
			LIMIT 1`
		err = r.db.QueryRowContext(ctx, query, msisdn, tenantID).Scan(&token)
	}
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return token, true, nil
}

// PreferredChannel resolves msisdn's notification-channel preference for the
// product identified by partnerRoleID/productID, mirroring the campaign
// lookup subscription-external uses to derive a feed item's product_slug.
// Returns "" (not an error) when no matching campaign or no stored
// preference exists; callers treat "" as SMS.
//
// Unlike app_devices, app_notification_prefs carries no tenant_key column
// (see migration 024), so it needs no separate tenant filter here: campaign
// slugs are globally unique (see the campaign slug/tenant resolution work),
// and the campaign this method joins through is already scoped to the
// caller's own partnerRoleID/productID, so anp.product_slug can only match
// the one campaign that pair resolves to.
func (r *PushRepository) PreferredChannel(ctx context.Context, msisdn string, partnerRoleID, productID int) (string, error) {
	const query = `
		SELECT anp.channel
		FROM app_notification_prefs anp
		JOIN LATERAL (
			SELECT slug FROM campaigns
			WHERE offer_product_id = $2
			  AND (partner_role_id IS NULL OR partner_role_id = $3)
			  AND enabled = true
			ORDER BY created_at DESC
			LIMIT 1
		) camp ON true
		WHERE anp.msisdn = $1 AND anp.product_slug = camp.slug`

	var channel string
	err := r.db.QueryRowContext(ctx, query, msisdn, productID, partnerRoleID).Scan(&channel)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return channel, nil
}
