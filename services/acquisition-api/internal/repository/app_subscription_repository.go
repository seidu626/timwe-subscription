package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
)

// nextChargeHint renders a human next-charge date for an ACTIVE subscription
// by advancing the last charge (or the opt-in date when nothing has been
// charged yet) by whole billing cycles until it lands after now. Unknown or
// missing cycles return "" and the field is omitted from the response.
func nextChargeHint(lastCharge time.Time, billingCycle string, now time.Time) string {
	if lastCharge.IsZero() {
		return ""
	}
	var advance func(time.Time) time.Time
	switch strings.ToLower(strings.TrimSpace(billingCycle)) {
	case "daily":
		advance = func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }
	case "weekly":
		advance = func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }
	case "biweekly":
		advance = func(t time.Time) time.Time { return t.AddDate(0, 0, 14) }
	case "monthly":
		advance = func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }
	default:
		return ""
	}
	next := advance(lastCharge)
	for i := 0; !next.After(now) && i < 10000; i++ {
		next = advance(next)
	}
	return "Renews " + next.Format("2 Jan")
}

// ListAppSubscriptionsByMSISDN lists a Dayline app user's subscriptions
// across every tenant (acquisition_transactions joined to campaigns for
// display/pricing fields and to tenants for marketplace attribution),
// newest first. Used by GET /v1/app/subscriptions: app identity is the
// msisdn, so the marketplace lists one combined view rather than a
// per-tenant slice.
func (r *TransactionRepository) ListAppSubscriptionsByMSISDN(msisdn string) ([]*domain.AppSubscription, error) {
	query := `
		SELECT t.id, t.campaign_slug, t.status, t.created_at, t.charged_at,
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
			chargedAt                                 sql.NullTime
		)
		var sub domain.AppSubscription
		if err := rows.Scan(&ref, &slug, &status, &sub.StartedAt, &chargedAt,
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
		if sub.Status == "ACTIVE" {
			lastCharge := sub.StartedAt
			if chargedAt.Valid {
				lastCharge = chargedAt.Time
			}
			sub.NextChargeHint = nextChargeHint(lastCharge, billingCycle.String, time.Now())
		}
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
