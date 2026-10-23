package repository

import (
	"context"
	"fmt"
	"time"
)

// expireStalePendingQuery ends one batch of pending/preactive rows that have
// not been written for longer than $1 seconds and have no DELIVERED
// CHARGE/USER_RENEWED notification for the same tenant, msisdn and product
// since the row was created. updated_at is the age column because an opt-in
// on an existing row keeps created_at and start_date; only the
// update_subscriptions_updated_at trigger moves updated_at. subscriptions
// timestamps are timestamp without time zone holding UTC; notifications
// created_at is timestamptz. SKIP LOCKED and the batch LIMIT keep each
// statement short on this heavily indexed table.
const expireStalePendingQuery = `
		WITH stale AS (
			SELECT s.id
			FROM subscriptions s
			WHERE s.status IN ('pending', 'preactive')
			  AND s.tenant_id IS NOT NULL
			  AND s.updated_at < (NOW() AT TIME ZONE 'UTC') - make_interval(secs => $1)
			  AND NOT EXISTS (
				SELECT 1
				FROM notifications n
				WHERE n.tenant_id = s.tenant_id
				  AND n.msisdn = s.user_identifier
				  AND n.product_id = s.product_id
				  AND n.type IN ('CHARGE', 'USER_RENEWED')
				  AND upper(trim(n.mno_delivery_code)) = 'DELIVERED'
				  AND n.created_at >= (s.created_at AT TIME ZONE 'UTC') - interval '5 minutes'
			  )
			ORDER BY s.id
			LIMIT $2
			FOR UPDATE OF s SKIP LOCKED
		)
		UPDATE subscriptions s
		SET status = 'inactive', end_date = NOW()
		FROM stale
		WHERE s.id = stale.id
		  AND s.status IN ('pending', 'preactive')`

// ExpireStalePendingSubscriptions ends up to limit pending/preactive rows
// older than window that never received payment evidence, returning the
// number of rows ended. A later DELIVERED charge reactivates an ended row
// through the notification projection.
func (r *SubscriptionRepository) ExpireStalePendingSubscriptions(ctx context.Context, window time.Duration, limit int) (int64, error) {
	if window <= 0 || limit <= 0 {
		return 0, fmt.Errorf("invalid pending expiry window %s or limit %d", window, limit)
	}
	result, err := r.db.ExecContext(ctx, expireStalePendingQuery, window.Seconds(), limit)
	if err != nil {
		return 0, fmt.Errorf("failed to expire stale pending subscriptions: %w", err)
	}
	expired, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to count expired pending subscriptions: %w", err)
	}
	return expired, nil
}
