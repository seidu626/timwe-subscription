package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
)

// TIMWEReportSource binds one provider report filter to an active tenant. The
// report itself has no tenant or channel identifier that our API can trust.
type TIMWEReportSource struct {
	Key       string `json:"key"`
	TenantKey string `json:"tenant_key"`
	Shortcode string `json:"shortcode"`
	ProductID int    `json:"product_id"`
	PartnerID int    `json:"partner_id"`
	TenantID  string `json:"-"`
}

type TIMWEReportRow struct {
	Day             time.Time
	PartnerID       int
	Shortcode       string
	ProductID       int
	ProductName     string
	PricepointID    int
	PricepointValue string
	SuccessBillings int64
	Revenue         string
}

func (r *ReportsRepository) ResolveTIMWETenant(ctx context.Context, tenantKey string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id::text FROM tenants WHERE tenant_key = $1 AND status = 'ACTIVE'`, tenantKey).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("resolve report tenant: %w", err)
	}
	return id, nil
}

// ReplaceTIMWEReport atomically replaces every row for the requested window.
// An empty but valid CSV is a successful zero report; a failure before commit
// leaves the prior window and last_success_at untouched.
func (r *ReportsRepository) ReplaceTIMWEReport(ctx context.Context, source TIMWEReportSource, start, end time.Time, rows []TIMWEReportRow, at time.Time) error {
	if source.TenantID == "" || source.Key == "" || start.After(end) {
		return errors.New("invalid report source or date window")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin report import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO timwe_report_syncs (tenant_id, source_key, shortcode, product_id, partner_id)
		VALUES ($1::uuid, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, source_key) DO NOTHING`, source.TenantID, source.Key, source.Shortcode, source.ProductID, source.PartnerID)
	if err != nil {
		return fmt.Errorf("create report source: %w", err)
	}
	var savedShortcode string
	var savedProduct, savedPartner int
	err = tx.QueryRowContext(ctx, `SELECT shortcode, product_id, partner_id FROM timwe_report_syncs WHERE tenant_id = $1::uuid AND source_key = $2 FOR UPDATE`, source.TenantID, source.Key).Scan(&savedShortcode, &savedProduct, &savedPartner)
	if err != nil {
		return fmt.Errorf("lock report source: %w", err)
	}
	if savedShortcode != source.Shortcode || savedProduct != source.ProductID || savedPartner != source.PartnerID {
		return errors.New("report source key already belongs to different filters")
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM timwe_report_daily WHERE tenant_id = $1::uuid AND source_key = $2 AND report_day BETWEEN $3::date AND $4::date`, source.TenantID, source.Key, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("replace report window: %w", err)
	}
	for _, row := range rows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO timwe_report_daily
			(tenant_id, source_key, report_day, partner_id, shortcode, product_id, product_name, pricepoint_id, pricepoint_value, success_billings, revenue, imported_at)
			VALUES ($1::uuid, $2, $3::date, $4, $5, $6, $7, $8, $9::numeric, $10, $11::numeric, $12)`,
			source.TenantID, source.Key, row.Day.Format("2006-01-02"), row.PartnerID, row.Shortcode, row.ProductID, row.ProductName, row.PricepointID, row.PricepointValue, row.SuccessBillings, row.Revenue, at)
		if err != nil {
			return fmt.Errorf("insert report day: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE timwe_report_syncs SET window_start = $3::date, window_end = $4::date,
			last_attempt_at = $5, last_success_at = $5, last_error_code = NULL
		WHERE tenant_id = $1::uuid AND source_key = $2`, source.TenantID, source.Key, start.Format("2006-01-02"), end.Format("2006-01-02"), at)
	if err != nil {
		return fmt.Errorf("record report success: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit report import: %w", err)
	}
	return nil
}

func (r *ReportsRepository) RecordTIMWEReportFailure(ctx context.Context, source TIMWEReportSource, code string, at time.Time) error {
	if source.TenantID == "" {
		return errors.New("report tenant is required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO timwe_report_syncs (tenant_id, source_key, shortcode, product_id, partner_id, last_attempt_at, last_error_code)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, source_key) DO UPDATE SET
			last_attempt_at = EXCLUDED.last_attempt_at, last_error_code = EXCLUDED.last_error_code
		WHERE timwe_report_syncs.shortcode = EXCLUDED.shortcode
		  AND timwe_report_syncs.product_id = EXCLUDED.product_id
		  AND timwe_report_syncs.partner_id = EXCLUDED.partner_id`,
		source.TenantID, source.Key, source.Shortcode, source.ProductID, source.PartnerID, at, code)
	if err != nil {
		return fmt.Errorf("record report failure: %w", err)
	}
	return nil
}

// GetSubscriptionHealth counts current tenant subscription records and leaves
// provider values nullable when no successful import exists.
func (r *ReportsRepository) GetSubscriptionHealth(ctx context.Context, tenantID string, start, end time.Time, productID int, shortcode string, staleAfter time.Duration) (*domain.SubscriptionHealthResponse, error) {
	if strings.TrimSpace(tenantID) == "" || start.After(end) {
		return nil, errors.New("tenant and valid date window are required")
	}
	result := &domain.SubscriptionHealthResponse{AsOf: time.Now().UTC(), Timezone: "Africa/Accra"}
	result.Provider = domain.TIMWEProviderSummary{State: "unavailable", Daily: []domain.TIMWEDailyItem{}}
	q := `SELECT COUNT(*) FILTER (WHERE lower(s.status) = 'active'),
		COUNT(*) FILTER (WHERE lower(s.status) IN ('inactive', 'cancelled', 'unsubscribed')),
		COUNT(*) FILTER (WHERE lower(COALESCE(s.status, 'unknown')) NOT IN ('active', 'inactive', 'cancelled', 'unsubscribed')),
		COUNT(*) FROM subscriptions s
		WHERE s.tenant_id = $1::uuid
		  AND ($2::integer = 0 OR s.product_id = $2)
		  AND ($3::text = '' OR EXISTS (
			SELECT 1 FROM products p WHERE p.tenant_id = s.tenant_id
			AND p.product_id = s.product_id::text AND p.short_code = $3))`
	if err := r.db.QueryRowContext(ctx, q, tenantID, productID, shortcode).Scan(&result.Subscriptions.Active, &result.Subscriptions.Inactive, &result.Subscriptions.Other, &result.Subscriptions.Total); err != nil {
		return nil, fmt.Errorf("query subscription state: %w", err)
	}

	statusRows, err := r.db.QueryContext(ctx, `
		SELECT last_success_at, last_error_code, window_start, window_end
		FROM timwe_report_syncs
		WHERE tenant_id = $1::uuid AND ($2::integer = 0 OR product_id = $2)
		  AND ($3::text = '' OR shortcode = $3)`, tenantID, productID, shortcode)
	if err != nil {
		return nil, fmt.Errorf("query report freshness: %w", err)
	}
	var sources, successful int
	stale := false
	for statusRows.Next() {
		sources++
		var success sql.NullTime
		var failure sql.NullString
		var windowStart, windowEnd sql.NullTime
		if err := statusRows.Scan(&success, &failure, &windowStart, &windowEnd); err != nil {
			_ = statusRows.Close()
			return nil, fmt.Errorf("scan report freshness: %w", err)
		}
		if !success.Valid {
			stale = true
			continue
		}
		successful++
		if result.Provider.LastImportedAt == nil || success.Time.Before(*result.Provider.LastImportedAt) {
			at := success.Time.UTC()
			result.Provider.LastImportedAt = &at
		}
		if failure.Valid || result.AsOf.Sub(success.Time) > staleAfter || !windowStart.Valid || !windowEnd.Valid || start.Before(windowStart.Time) || end.After(windowEnd.Time) {
			stale = true
		}
	}
	if err := statusRows.Err(); err != nil {
		_ = statusRows.Close()
		return nil, fmt.Errorf("read report freshness: %w", err)
	}
	_ = statusRows.Close()
	if sources == 0 || successful == 0 {
		return result, nil
	}
	result.Provider.State = "fresh"
	if stale || successful != sources {
		result.Provider.State = "stale"
	}

	var billings int64
	var revenue string
	err = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(success_billings), 0), COALESCE(SUM(revenue), 0)::text
		FROM timwe_report_daily WHERE tenant_id = $1::uuid AND report_day BETWEEN $2::date AND $3::date
		AND ($4::integer = 0 OR product_id = $4) AND ($5::text = '' OR shortcode = $5)`,
		tenantID, start.Format("2006-01-02"), end.Format("2006-01-02"), productID, shortcode).Scan(&billings, &revenue)
	if err != nil {
		return nil, fmt.Errorf("query report totals: %w", err)
	}
	result.Provider.SuccessBillings = &billings
	result.Provider.Revenue = &revenue

	dailyRows, err := r.db.QueryContext(ctx, `
		SELECT report_day, shortcode, product_id, product_name, pricepoint_id,
			SUM(success_billings), SUM(revenue)::text
		FROM timwe_report_daily
		WHERE tenant_id = $1::uuid AND report_day BETWEEN $2::date AND $3::date
		  AND ($4::integer = 0 OR product_id = $4) AND ($5::text = '' OR shortcode = $5)
		GROUP BY report_day, shortcode, product_id, product_name, pricepoint_id
		ORDER BY report_day DESC, product_id, pricepoint_id`,
		tenantID, start.Format("2006-01-02"), end.Format("2006-01-02"), productID, shortcode)
	if err != nil {
		return nil, fmt.Errorf("query report days: %w", err)
	}
	defer dailyRows.Close()
	for dailyRows.Next() {
		var item domain.TIMWEDailyItem
		var day time.Time
		if err := dailyRows.Scan(&day, &item.Shortcode, &item.ProductID, &item.ProductName, &item.PricepointID, &item.SuccessBillings, &item.Revenue); err != nil {
			return nil, fmt.Errorf("scan report day: %w", err)
		}
		item.Date = day.Format("2006-01-02")
		result.Provider.Daily = append(result.Provider.Daily, item)
	}
	if err := dailyRows.Err(); err != nil {
		return nil, fmt.Errorf("read report days: %w", err)
	}
	return result, nil
}
