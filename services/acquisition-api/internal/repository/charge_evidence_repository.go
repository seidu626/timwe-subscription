package repository

import (
	"fmt"

	"github.com/google/uuid"
)

// NoChargeEvidenceFailureReason marks transactions the charge reconciler
// expired because no DELIVERED charge arrived within the expiry window.
const NoChargeEvidenceFailureReason = "NO_CHARGE_EVIDENCE"

// chargeEvidenceExists matches a DELIVERED CHARGE/USER_RENEWED notification
// for transaction alias t. TIMWE's transaction_uuid does not reliably equal
// timwe_transaction_id, so the match is tenant + msisdn + product + time.
// acquisition_transactions.created_at is timestamp without time zone holding
// UTC; notifications.created_at is timestamptz.
const chargeEvidenceExists = `EXISTS (
		SELECT 1
		FROM notifications n
		WHERE n.msisdn = t.msisdn
		  AND n.product_id = t.offer_product_id
		  AND n.tenant_id = t.tenant_id
		  AND n.type IN ('CHARGE', 'USER_RENEWED')
		  AND upper(trim(n.mno_delivery_code)) = 'DELIVERED'
		  AND n.created_at >= (t.created_at AT TIME ZONE 'UTC') - interval '5 minutes'
	)`

// ChargeEvidenceCandidate identifies an uncharged transaction with payment
// evidence in the notifications table.
type ChargeEvidenceCandidate struct {
	ID       uuid.UUID
	TenantID string
}

// ListChargeEvidenceCandidates returns up to limit uncharged transactions
// created within lookbackDays that have DELIVERED charge evidence. Only the
// newest transaction per (tenant_id, msisdn, offer_product_id) qualifies, so
// one charge never promotes several opt-in attempts.
func (r *TransactionRepository) ListChargeEvidenceCandidates(lookbackDays, limit int) ([]ChargeEvidenceCandidate, error) {
	query := `
		SELECT t.id, t.tenant_id::text
		FROM acquisition_transactions t
		WHERE t.charged_at IS NULL
		  AND t.tenant_id IS NOT NULL
		  AND t.offer_product_id IS NOT NULL
		  AND t.msisdn <> ''
		  AND t.created_at >= (now() AT TIME ZONE 'UTC') - ($1::int * interval '1 day')
		  AND (
		        t.status IN ('SUBSCRIBED', 'CONFIRM_REQUIRED', 'ACTION_REQUIRED')
		     OR (t.status = 'FAILED' AND t.next_action_payload->>'failure_reason' = '` + NoChargeEvidenceFailureReason + `')
		  )
		  AND NOT EXISTS (
		        SELECT 1
		        FROM acquisition_transactions newer
		        WHERE newer.tenant_id = t.tenant_id
		          AND newer.msisdn = t.msisdn
		          AND newer.offer_product_id = t.offer_product_id
		          AND (newer.created_at > t.created_at
		               OR (newer.created_at = t.created_at AND newer.id > t.id))
		  )
		  AND ` + chargeEvidenceExists + `
		ORDER BY t.created_at ASC
		LIMIT $2
	`
	rows, err := r.db.Query(query, lookbackDays, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list charge evidence candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]ChargeEvidenceCandidate, 0)
	for rows.Next() {
		var c ChargeEvidenceCandidate
		if err := rows.Scan(&c.ID, &c.TenantID); err != nil {
			return nil, fmt.Errorf("failed to scan charge evidence candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate charge evidence candidates: %w", err)
	}
	return candidates, nil
}

// ExpireUnchargedTransactions moves pre-charge transactions older than
// expiryDays with no DELIVERED charge evidence to FAILED with failure_reason
// NO_CHARGE_EVIDENCE, returning the number of rows expired.
func (r *TransactionRepository) ExpireUnchargedTransactions(expiryDays int) (int64, error) {
	query := `
		UPDATE acquisition_transactions t
		SET status = 'FAILED',
		    next_action = NULL,
		    next_action_payload = jsonb_build_object('failure_reason', '` + NoChargeEvidenceFailureReason + `', 'expired_at', now()),
		    updated_at = CURRENT_TIMESTAMP
		WHERE t.status IN ('SUBSCRIBED', 'CONFIRM_REQUIRED', 'ACTION_REQUIRED')
		  AND t.charged_at IS NULL
		  AND t.created_at < (now() AT TIME ZONE 'UTC') - ($1::int * interval '1 day')
		  AND NOT ` + chargeEvidenceExists + `
	`
	result, err := r.db.Exec(query, expiryDays)
	if err != nil {
		return 0, fmt.Errorf("failed to expire uncharged transactions: %w", err)
	}
	expired, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to count expired transactions: %w", err)
	}
	return expired, nil
}
