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

// DeviceToken returns the most recently registered FCM token for msisdn.
// found is false when msisdn has no registered device (not an error).
func (r *PushRepository) DeviceToken(ctx context.Context, msisdn string) (string, bool, error) {
	const query = `
		SELECT fcm_token
		FROM app_devices
		WHERE msisdn = $1
		ORDER BY updated_at DESC
		LIMIT 1`

	var token string
	err := r.db.QueryRowContext(ctx, query, msisdn).Scan(&token)
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
