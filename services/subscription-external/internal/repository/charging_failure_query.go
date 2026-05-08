// charging_failure_query.go - Updated for Notifications-Based Strategy
// File: internal/repository/charging_failure_query.go
// Based on FINAL_CHARGING_STRATEGY.md

package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ChargingFailureFilter contains criteria for finding failed subscriptions
type ChargingFailureFilter struct {
	ProductIDs       []int
	StartDate        *time.Time
	EndDate          *time.Time
	MinFailureCount  int
	ExcludeProcessed bool
	Limit            int
	Offset           int
	BatchID          string
	DaysThreshold    int // Days threshold for charging failures
}

// ChargingFailedSubscription represents a subscription with charging issues
type ChargingFailedSubscription struct {
	ID                       int
	MSISDN                   string
	ProductID                int
	EntryChannel             string
	Status                   string
	SubscriptionDate         time.Time
	LastChargeDate           *time.Time
	LastOptinDate            *time.Time
	DaysWithoutCharge        int
	ChargingStatus           string
	OptinChargeStatus        string
	TotalChargeNotifications int
	ChargingHealthStatus     string
	ChargingFailureReason    string
}

// FetchChargingFailedSubscriptions retrieves subscriptions with charging issues
// Updated to use the charging_failed_subscriptions view for better performance
func (r *SubscriptionRepository) FetchChargingFailedSubscriptions(filter ChargingFailureFilter) ([]ChargingFailedSubscription, error) {
	var conditions []string
	var args []interface{}
	argCount := 0

	// Base query using the charging_failed_subscriptions view
	baseQuery := `
        SELECT 
            subscription_id as id,
            msisdn,
            product_id,
            entry_channel,
            'active' as status,
            subscription_date,
            last_charge,
            last_optin,
            days_since_subscription as days_without_charge,
            charging_status,
            optin_charge_status,
            total_charges as total_charge_notifications,
            charging_status as charging_health_status,
            '' as charging_failure_reason
        FROM charging_failed_subscriptions
        WHERE 1=1
    `

	// Add product filter
	if len(filter.ProductIDs) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("product_id = ANY($%d)", argCount))
		args = append(args, filter.ProductIDs)
	}

	// Add date range filter
	if filter.StartDate != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("subscription_date >= $%d", argCount))
		args = append(args, *filter.StartDate)
	}

	if filter.EndDate != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("subscription_date <= $%d", argCount))
		args = append(args, *filter.EndDate)
	}

	// Add days threshold filter - temporarily disabled for debugging
	// if filter.DaysThreshold > 0 {
	// 	argCount++
	// 	conditions = append(conditions, fmt.Sprintf("days_since_subscription >= $%d", argCount))
	// 	args = append(args, filter.DaysThreshold)
	// }

	// Add exclude processed filter - Note: resubscribe_status not available in view
	// TODO: Implement this filter when resubscribe_status is added to subscriptions table
	if filter.ExcludeProcessed {
		r.logger.Warn("ExcludeProcessed filter requested but resubscribe_status not available in view")
		// For now, we'll skip this filter since the column doesn't exist in the view
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	// Complete query
	query := baseQuery + whereClause + " ORDER BY days_without_charge DESC"

	// Add limit and offset
	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	// Execute query
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch charging failed subscriptions: %w", err)
	}
	defer rows.Close()

	var failures []ChargingFailedSubscription
	for rows.Next() {
		var f ChargingFailedSubscription
		var lastChargeDate, lastOptinDate sql.NullTime
		var totalChargeNotifications sql.NullInt64
		var chargingHealthStatus, chargingFailureReason sql.NullString

		err := rows.Scan(
			&f.ID,
			&f.MSISDN,
			&f.ProductID,
			&f.EntryChannel,
			&f.Status,
			&f.SubscriptionDate,
			&lastChargeDate,
			&lastOptinDate,
			&f.DaysWithoutCharge,
			&f.ChargingStatus,
			&f.OptinChargeStatus,
			&totalChargeNotifications,
			&chargingHealthStatus,
			&chargingFailureReason,
		)
		if err != nil {
			r.logger.Error("Failed to scan charging failure row", zap.Error(err))
			continue
		}

		// Handle nullable fields
		if lastChargeDate.Valid {
			f.LastChargeDate = &lastChargeDate.Time
		}
		if lastOptinDate.Valid {
			f.LastOptinDate = &lastOptinDate.Time
		}
		if totalChargeNotifications.Valid {
			f.TotalChargeNotifications = int(totalChargeNotifications.Int64)
		} else {
			f.TotalChargeNotifications = 0
		}
		if chargingHealthStatus.Valid {
			f.ChargingHealthStatus = chargingHealthStatus.String
		}
		if chargingFailureReason.Valid {
			f.ChargingFailureReason = chargingFailureReason.String
		}

		failures = append(failures, f)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating charging failure rows: %w", err)
	}

	r.logger.Info("Fetched charging failed subscriptions",
		zap.Int("count", len(failures)),
		zap.Int("limit", filter.Limit),
		zap.Int("offset", filter.Offset))

	return failures, nil
}

// GetChargingFailureCount returns the total count of subscriptions with charging issues
func (r *SubscriptionRepository) GetChargingFailureCount(filter ChargingFailureFilter) (int64, error) {
	var conditions []string
	var args []interface{}
	argCount := 0

	// Base count query using the charging_failed_subscriptions view
	baseQuery := `
        SELECT COUNT(*)
        FROM charging_failed_subscriptions
        WHERE 1=1
    `

	// Add product filter
	if len(filter.ProductIDs) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("product_id = ANY($%d)", argCount))
		args = append(args, filter.ProductIDs)
	}

	// Add date range filter
	if filter.StartDate != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("subscription_date >= $%d", argCount))
		args = append(args, *filter.StartDate)
	}

	if filter.EndDate != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("subscription_date <= $%d", argCount))
		args = append(args, *filter.EndDate)
	}

	// Add days threshold filter - temporarily disabled for debugging
	// if filter.DaysThreshold > 0 {
	// 	argCount++
	// 	conditions = append(conditions, fmt.Sprintf("days_since_subscription >= $%d", argCount))
	// 	args = append(args, filter.DaysThreshold)
	// }

	// Add exclude processed filter - Note: resubscribe_status not available in view
	// TODO: Implement this filter when resubscribe_status is added to subscriptions table
	if filter.ExcludeProcessed {
		r.logger.Warn("ExcludeProcessed filter requested but resubscribe_status not available in view")
		// For now, we'll skip this filter since the column doesn't exist in the view
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	query := baseQuery + whereClause

	var count int64
	err := r.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get charging failure count: %w", err)
	}

	return count, nil
}

// GetChargingFailureStats returns statistics about charging failures
// Uses get_charging_failure_stats_fast() for better performance (pre-computed column)
// Falls back to get_charging_failure_stats() if fast function unavailable
func (r *SubscriptionRepository) GetChargingFailureStats() (map[string]interface{}, error) {
	// Try the fast function first (uses pre-computed charging_health_status column)
	stats, err := r.getChargingFailureStatsFast()
	if err == nil {
		return stats, nil
	}

	r.logger.Warn("Fast stats function unavailable, falling back to standard function",
		zap.Error(err))

	// Fallback to standard function
	return r.getChargingFailureStatsStandard()
}

// getChargingFailureStatsFast uses the optimized function with pre-computed column
func (r *SubscriptionRepository) getChargingFailureStatsFast() (map[string]interface{}, error) {
	query := `
        SELECT 
            category,
            count,
            percentage
        FROM get_charging_failure_stats_fast()
        ORDER BY 
            CASE category
                WHEN 'Total Subscriptions' THEN 1
                WHEN 'Total Charging Failures' THEN 2
                WHEN 'Never Charged' THEN 3
                WHEN 'Stale Charges (>30 days)' THEN 4
                WHEN 'Charging Delayed (7-30 days)' THEN 5
                WHEN 'Charging Recent (<7 days)' THEN 6
                ELSE 7
            END
    `

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("fast stats function not available: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]interface{})
	var totalFailures int64

	for rows.Next() {
		var category string
		var count int64
		var percentage float64

		err := rows.Scan(&category, &count, &percentage)
		if err != nil {
			r.logger.Error("Failed to scan charging failure stats row", zap.Error(err))
			continue
		}

		stats[category] = map[string]interface{}{
			"count":      count,
			"percentage": percentage,
		}

		if category == "Total Charging Failures" {
			totalFailures = count
		}
	}

	// Add total failures for compatibility
	stats["total_failures"] = totalFailures
	stats["total_charging_failures"] = totalFailures

	r.logger.Info("Retrieved charging failure stats using fast function",
		zap.Int64("total_failures", totalFailures))

	return stats, nil
}

// getChargingFailureStatsStandard uses the original function (fallback)
func (r *SubscriptionRepository) getChargingFailureStatsStandard() (map[string]interface{}, error) {
	query := `
        SELECT 
            category,
            count,
            percentage
        FROM get_charging_failure_stats()
        ORDER BY 
            CASE category
                WHEN 'Total Charging Failures' THEN 1
                WHEN 'Never Charged' THEN 2
                WHEN 'Stale Charges (>30 days)' THEN 3
                WHEN 'No Optin' THEN 4
                WHEN 'Optin but No Charge' THEN 5
                ELSE 6
            END
    `

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get charging failure stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]interface{})
	var totalFailures int64

	for rows.Next() {
		var category string
		var count int64
		var percentage float64

		err := rows.Scan(&category, &count, &percentage)
		if err != nil {
			r.logger.Error("Failed to scan charging failure stats row", zap.Error(err))
			continue
		}

		stats[category] = map[string]interface{}{
			"count":      count,
			"percentage": percentage,
		}

		if category != "Total Charging Failures" {
			totalFailures += count
		}
	}

	// Add total failures
	stats["total_failures"] = totalFailures

	// Get total count using function
	var totalCount int64
	err = r.db.QueryRow("SELECT get_total_charging_failures()").Scan(&totalCount)
	if err != nil {
		r.logger.Warn("Failed to get total charging failures count", zap.Error(err))
	} else {
		stats["total_charging_failures"] = totalCount
	}

	return stats, nil
}

// GetChargingFailureSummary returns a summary view of charging failures
func (r *SubscriptionRepository) GetChargingFailureSummary() (map[string]interface{}, error) {
	query := `
        SELECT 
            charging_health_status,
            subscription_count,
            percentage
        FROM charging_failure_summary
        ORDER BY 
            CASE charging_health_status
                WHEN 'NEVER_CHARGED' THEN 1
                WHEN 'CHARGING_STALE' THEN 2
                WHEN 'CHARGING_DELAYED' THEN 3
                WHEN 'CHARGING_RECENT' THEN 4
                ELSE 5
            END
    `

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get charging failure summary: %w", err)
	}
	defer rows.Close()

	summary := make(map[string]interface{})
	for rows.Next() {
		var status string
		var count int64
		var percentage float64

		err := rows.Scan(&status, &count, &percentage)
		if err != nil {
			r.logger.Error("Failed to scan charging failure summary row", zap.Error(err))
			continue
		}

		summary[status] = map[string]interface{}{
			"count":      count,
			"percentage": percentage,
		}
	}

	return summary, nil
}

// UpdateChargingHealthStatus updates the charging health status for a subscription
func (r *SubscriptionRepository) UpdateChargingHealthStatus(subscriptionID int, status string, reason string) error {
	query := `
        UPDATE subscriptions 
        SET 
            charging_health_status = $1,
            charging_failure_reason = $2,
            last_charging_failure_at = CURRENT_TIMESTAMP
        WHERE id = $3
    `

	_, err := r.db.Exec(query, status, reason, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to update charging health status: %w", err)
	}

	return nil
}

// MarkChargingFailureAsProcessed marks a charging failure as processed
func (r *SubscriptionRepository) MarkChargingFailureAsProcessed(subscriptionID int, status string) error {
	query := `
        UPDATE subscriptions 
        SET 
            resubscribe_status = $1,
            last_resubscribe_attempt_at = CURRENT_TIMESTAMP,
            resubscribe_attempt_count = COALESCE(resubscribe_attempt_count, 0) + 1
        WHERE id = $2
    `

	_, err := r.db.Exec(query, status, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to mark charging failure as processed: %w", err)
	}

	return nil
}

// GetChargingFailureByMSISDN retrieves charging failure information for a specific MSISDN
func (r *SubscriptionRepository) GetChargingFailureByMSISDN(msisdn string, productID int) (*ChargingFailedSubscription, error) {
	query := `
        WITH charge_data AS (
            SELECT 
                n.msisdn,
                n.product_id,
                MAX(n.created_at) as last_charge,
                COUNT(*) as total_charges
            FROM notifications n
            WHERE n.type IN ('CHARGE', 'USER_RENEWED')
            GROUP BY n.msisdn, n.product_id
        ),
        optin_data AS (
            SELECT 
                n.msisdn,
                n.product_id,
                MAX(n.created_at) as last_optin
            FROM notifications n
            WHERE n.type = 'USER_OPTIN'
            GROUP BY n.msisdn, n.product_id
        )
        SELECT 
            s.id,
            s.user_identifier as msisdn,
            s.product_id,
            s.entry_channel,
            s.status,
            s.created_at as subscription_date,
            cd.last_charge as last_charge_date,
            od.last_optin as last_optin_date,
            CASE 
                WHEN cd.last_charge IS NULL THEN 
                    EXTRACT(DAY FROM NOW() - s.created_at)::INTEGER
                ELSE 
                    EXTRACT(DAY FROM NOW() - cd.last_charge)::INTEGER
            END as days_without_charge,
            CASE
                WHEN cd.last_charge IS NULL THEN 'NEVER_CHARGED'
                WHEN cd.last_charge < NOW() - INTERVAL '30 days' THEN 'STALE_CHARGE'
                ELSE 'RECENT_CHARGE'
            END as charging_status,
            CASE
                WHEN od.last_optin IS NULL THEN 'NO_OPTIN'
                WHEN cd.last_charge IS NULL THEN 'OPTIN_NO_CHARGE'
                WHEN cd.last_charge < NOW() - INTERVAL '30 days' THEN 'OPTIN_STALE_CHARGE'
                ELSE 'OPTIN_RECENT_CHARGE'
            END as optin_charge_status,
            COALESCE(cd.total_charges, 0) as total_charge_notifications,
            s.charging_health_status,
            s.charging_failure_reason
        FROM subscriptions s
        LEFT JOIN charge_data cd ON s.user_identifier = cd.msisdn 
                                  AND s.product_id = cd.product_id
        LEFT JOIN optin_data od ON s.user_identifier = od.msisdn 
                                  AND s.product_id = od.product_id
        WHERE s.user_identifier = $1 AND s.product_id = $2
          AND (s.status = 'active' OR s.status IS NULL)
          AND s.created_at < NOW() - INTERVAL '1 day'
          AND (cd.last_charge IS NULL 
               OR cd.last_charge < NOW() - INTERVAL '30 days')
    `

	var f ChargingFailedSubscription
	var lastChargeDate, lastOptinDate sql.NullTime
	var chargingHealthStatus, chargingFailureReason sql.NullString

	err := r.db.QueryRow(query, msisdn, productID).Scan(
		&f.ID,
		&f.MSISDN,
		&f.ProductID,
		&f.EntryChannel,
		&f.Status,
		&f.SubscriptionDate,
		&lastChargeDate,
		&lastOptinDate,
		&f.DaysWithoutCharge,
		&f.ChargingStatus,
		&f.OptinChargeStatus,
		&f.TotalChargeNotifications,
		&chargingHealthStatus,
		&chargingFailureReason,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No charging failure found
		}
		return nil, fmt.Errorf("failed to get charging failure by MSISDN: %w", err)
	}

	// Handle nullable fields
	if lastChargeDate.Valid {
		f.LastChargeDate = &lastChargeDate.Time
	}
	if lastOptinDate.Valid {
		f.LastOptinDate = &lastOptinDate.Time
	}
	if chargingHealthStatus.Valid {
		f.ChargingHealthStatus = chargingHealthStatus.String
	}
	if chargingFailureReason.Valid {
		f.ChargingFailureReason = chargingFailureReason.String
	}

	return &f, nil
}
