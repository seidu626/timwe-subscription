package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
)

// ListAppSubscriptionsByMSISDN lists a Dayline app user's subscriptions
// across every tenant (acquisition_transactions joined to campaigns for
// display/pricing fields and to tenants for marketplace attribution),
// newest first. Used by GET /v1/app/subscriptions: app identity is the
// msisdn, so the marketplace lists one combined view rather than a
// per-tenant slice.
func (r *TransactionRepository) ListAppSubscriptionsByMSISDN(msisdn string) ([]*domain.AppSubscription, error) {
	query := `
		SELECT t.id, t.campaign_slug, t.status, t.created_at,
		       c.price, c.billing_cycle, c.app_name, c.lp_copy, c.country,
		       tn.tenant_key, tn.name
		FROM acquisition_transactions t
		JOIN campaigns c ON c.slug = t.campaign_slug
		JOIN tenants tn ON tn.id = t.tenant_id
		WHERE t.msisdn = $1
		ORDER BY t.created_at DESC
	`
	rows, err := r.db.Query(query, msisdn)
	if err != nil {
		return nil, fmt.Errorf("failed to list app subscriptions: %w", err)
	}
	defer rows.Close()

	subs := make([]*domain.AppSubscription, 0)
	for rows.Next() {
		var (
			ref, slug, status, tenantKey, tenantName  string
			billingCycle, appName, lpCopy, rowCountry sql.NullString
			price                                     sql.NullFloat64
		)
		var sub domain.AppSubscription
		if err := rows.Scan(&ref, &slug, &status, &sub.StartedAt,
			&price, &billingCycle, &appName, &lpCopy, &rowCountry,
			&tenantKey, &tenantName); err != nil {
			continue
		}

		var lpEn *appLPCopyText
		if lpCopy.Valid && lpCopy.String != "" {
			var payload appLPCopyPayload
			if jsonErr := json.Unmarshal([]byte(lpCopy.String), &payload); jsonErr == nil {
				lpEn = payload.En
			}
		}

		sub.Ref = ref
		sub.Tenant = tenantKey
		sub.TenantName = tenantName
		sub.ProductSlug = slug
		sub.ProductName = firstNonEmpty(appName.String, lpTitleOf(lpEn), slug)
		sub.Status = domain.MapTransactionStatusToApp(domain.TransactionStatus(status))
		sub.BillingCycle = billingCycle.String
		sub.Currency = currencyForCountry(rowCountry.String)
		if price.Valid {
			v := price.Float64
			sub.Price = &v
		}
		subs = append(subs, &sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate app subscriptions: %w", err)
	}
	return subs, nil
}
