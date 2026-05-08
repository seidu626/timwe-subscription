package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// TransactionRepository handles acquisition transaction data access
type TransactionRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(db *sql.DB, logger *zap.Logger) *TransactionRepository {
	return &TransactionRepository{
		db:     db,
		logger: logger,
	}
}

// DB returns the underlying database connection for advanced queries
func (r *TransactionRepository) DB() *sql.DB {
	return r.db
}

// Create creates a new acquisition transaction
func (r *TransactionRepository) Create(tx *domain.AcquisitionTransaction) error {
	query := `
		INSERT INTO acquisition_transactions (
			id, correlation_id, tenant_id, campaign_slug, msisdn, status, next_action,
			next_action_payload, ad_provider, click_id, attribution_data,
			ip_address, user_agent, consent_required, consent_checked,
			consent_version, consent_timestamp, landing_version_hash,
			offer_product_id, pricepoint_id, partner_role_id,
			timwe_transaction_id, transaction_auth_code, timwe_status,
			he_source, he_msisdn, he_operator,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29
		)
	`

	var nextAction, adProvider, clickID, ipAddress, userAgent, consentVersion,
		landingVersionHash, tenantID, timweTransactionID,
		transactionAuthCode, timweStatus, heSource, heMSISDN, heOperator sql.NullString
	var offerProductID, pricepointID, partnerRoleID sql.NullInt64
	var consentTimestamp sql.NullTime
	var nextActionPayload sql.NullString
	var attributionData sql.NullString

	if tx.NextAction != nil {
		nextAction.String = string(*tx.NextAction)
		nextAction.Valid = true
	}
	if tx.TenantID != nil {
		tenantID.String = *tx.TenantID
		tenantID.Valid = true
	}
	if tx.AdProvider != nil {
		adProvider.String = *tx.AdProvider
		adProvider.Valid = true
	}
	if tx.ClickID != nil {
		clickID.String = *tx.ClickID
		clickID.Valid = true
	}
	if tx.IPAddress != nil {
		ipAddress.String = *tx.IPAddress
		ipAddress.Valid = true
	}
	if tx.UserAgent != nil {
		userAgent.String = *tx.UserAgent
		userAgent.Valid = true
	}
	if tx.ConsentVersion != nil {
		consentVersion.String = *tx.ConsentVersion
		consentVersion.Valid = true
	}
	if tx.ConsentTimestamp != nil {
		consentTimestamp.Time = *tx.ConsentTimestamp
		consentTimestamp.Valid = true
	}
	if tx.LandingVersionHash != nil {
		landingVersionHash.String = *tx.LandingVersionHash
		landingVersionHash.Valid = true
	}
	if tx.OfferProductID != nil {
		offerProductID.Int64 = int64(*tx.OfferProductID)
		offerProductID.Valid = true
	}
	if tx.PricepointID != nil {
		pricepointID.Int64 = int64(*tx.PricepointID)
		pricepointID.Valid = true
	}
	if tx.PartnerRoleID != nil {
		partnerRoleID.Int64 = int64(*tx.PartnerRoleID)
		partnerRoleID.Valid = true
	}
	if tx.TimweTransactionID != nil {
		timweTransactionID.String = *tx.TimweTransactionID
		timweTransactionID.Valid = true
	}
	if tx.TransactionAuthCode != nil {
		transactionAuthCode.String = *tx.TransactionAuthCode
		transactionAuthCode.Valid = true
	}
	if tx.TimweStatus != nil {
		timweStatus.String = *tx.TimweStatus
		timweStatus.Valid = true
	}
	if tx.HESource != nil {
		heSource.String = string(*tx.HESource)
		heSource.Valid = true
	}
	if tx.HEMSISDN != nil {
		heMSISDN.String = *tx.HEMSISDN
		heMSISDN.Valid = true
	}
	if tx.HEOperator != nil {
		heOperator.String = *tx.HEOperator
		heOperator.Valid = true
	}

	if len(tx.NextActionPayload) > 0 {
		nextActionPayload.String = string(tx.NextActionPayload)
		nextActionPayload.Valid = true
	}
	if len(tx.AttributionData) > 0 {
		attributionData.String = string(tx.AttributionData)
		attributionData.Valid = true
	}

	_, err := r.db.Exec(query,
		tx.ID, tx.CorrelationID, tenantID, tx.CampaignSlug, tx.MSISDN, tx.Status,
		nextAction, nextActionPayload, adProvider, clickID, attributionData,
		ipAddress, userAgent, tx.ConsentRequired, tx.ConsentChecked,
		consentVersion, consentTimestamp, landingVersionHash,
		offerProductID, pricepointID, partnerRoleID,
		timweTransactionID, transactionAuthCode, timweStatus,
		heSource, heMSISDN, heOperator,
		tx.CreatedAt, tx.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a transaction by ID
func (r *TransactionRepository) GetByID(id uuid.UUID) (*domain.AcquisitionTransaction, error) {
	query := `
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE id = $1
	`

	tx, err := r.scanTransaction(query, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *TransactionRepository) GetTenantIDByID(id uuid.UUID) (string, error) {
	var tenantID sql.NullString
	if err := r.db.QueryRow(`SELECT tenant_id FROM acquisition_transactions WHERE id = $1`, id).Scan(&tenantID); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("transaction not found")
		}
		return "", fmt.Errorf("failed to get transaction tenant: %w", err)
	}
	if !tenantID.Valid {
		return "", nil
	}
	return tenantID.String, nil
}

// UpdateStatus updates the transaction status and related fields
func (r *TransactionRepository) UpdateStatus(id uuid.UUID, status domain.TransactionStatus, nextAction *domain.NextAction, payload json.RawMessage) error {
	query := `
		UPDATE acquisition_transactions
		SET status = $1, next_action = $2, next_action_payload = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
	`

	var nextActionVal sql.NullString
	if nextAction != nil {
		nextActionVal.String = string(*nextAction)
		nextActionVal.Valid = true
	}

	var payloadVal sql.NullString
	if len(payload) > 0 {
		payloadVal.String = string(payload)
		payloadVal.Valid = true
	}

	_, err := r.db.Exec(query, status, nextActionVal, payloadVal, id)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	return nil
}

// UpdateTIMWEData updates TIMWE-related fields
func (r *TransactionRepository) UpdateTIMWEData(id uuid.UUID, transactionID, authCode, status string) error {
	query := `
		UPDATE acquisition_transactions
		SET timwe_transaction_id = $1, transaction_auth_code = $2, 
		    timwe_status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
	`

	_, err := r.db.Exec(query, transactionID, authCode, status, id)
	if err != nil {
		return fmt.Errorf("failed to update TIMWE data: %w", err)
	}

	return nil
}

// ScanTransaction scans a transaction from a query (exported for use by handlers)
func (r *TransactionRepository) ScanTransaction(query string, args ...interface{}) (*domain.AcquisitionTransaction, error) {
	return r.scanTransaction(query, args...)
}

// scanTransaction scans a transaction from a query
func (r *TransactionRepository) scanTransaction(query string, args ...interface{}) (*domain.AcquisitionTransaction, error) {
	var tx domain.AcquisitionTransaction
	var nextAction, adProvider, clickID, ipAddress, userAgent, consentVersion,
		landingVersionHash, timweTransactionID,
		transactionAuthCode, timweStatus, heSource, heMSISDN, heOperator, chargePayout sql.NullString
	var offerProductID, pricepointID, partnerRoleID sql.NullInt64
	var consentTimestamp, chargedAt sql.NullTime
	var nextActionPayload, attributionData sql.NullString

	err := r.db.QueryRow(query, args...).Scan(
		&tx.ID, &tx.CorrelationID, &tx.CampaignSlug, &tx.MSISDN, &tx.Status,
		&nextAction, &nextActionPayload, &adProvider, &clickID, &attributionData,
		&ipAddress, &userAgent, &tx.ConsentRequired, &tx.ConsentChecked,
		&consentVersion, &consentTimestamp, &landingVersionHash,
		&offerProductID, &pricepointID, &partnerRoleID,
		&timweTransactionID, &transactionAuthCode, &timweStatus,
		&heSource, &heMSISDN, &heOperator,
		&chargedAt, &chargePayout, &tx.ConversionPostbackSent,
		&tx.CreatedAt, &tx.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan transaction: %w", err)
	}

	// Map nullable fields
	if nextAction.Valid {
		action := domain.NextAction(nextAction.String)
		tx.NextAction = &action
	}
	if adProvider.Valid {
		tx.AdProvider = &adProvider.String
	}
	if clickID.Valid {
		tx.ClickID = &clickID.String
	}
	if ipAddress.Valid {
		tx.IPAddress = &ipAddress.String
	}
	if userAgent.Valid {
		tx.UserAgent = &userAgent.String
	}
	if consentVersion.Valid {
		tx.ConsentVersion = &consentVersion.String
	}
	if consentTimestamp.Valid {
		tx.ConsentTimestamp = &consentTimestamp.Time
	}
	if landingVersionHash.Valid {
		tx.LandingVersionHash = &landingVersionHash.String
	}
	if offerProductID.Valid {
		val := int(offerProductID.Int64)
		tx.OfferProductID = &val
	}
	if pricepointID.Valid {
		val := int(pricepointID.Int64)
		tx.PricepointID = &val
	}
	if partnerRoleID.Valid {
		val := int(partnerRoleID.Int64)
		tx.PartnerRoleID = &val
	}
	if timweTransactionID.Valid {
		tx.TimweTransactionID = &timweTransactionID.String
	}
	if transactionAuthCode.Valid {
		tx.TransactionAuthCode = &transactionAuthCode.String
	}
	if timweStatus.Valid {
		tx.TimweStatus = &timweStatus.String
	}
	if heSource.Valid {
		src := domain.HESource(heSource.String)
		tx.HESource = &src
	}
	if heMSISDN.Valid {
		tx.HEMSISDN = &heMSISDN.String
	}
	if heOperator.Valid {
		tx.HEOperator = &heOperator.String
	}
	if chargedAt.Valid {
		tx.ChargedAt = &chargedAt.Time
	}
	if chargePayout.Valid {
		tx.ChargePayout = &chargePayout.String
	}

	if nextActionPayload.Valid {
		tx.NextActionPayload = json.RawMessage(nextActionPayload.String)
	}
	if attributionData.Valid {
		tx.AttributionData = json.RawMessage(attributionData.String)
	}

	return &tx, nil
}

// CheckThrottle checks if a request should be throttled based on campaign rules
func (r *TransactionRepository) CheckThrottle(campaignSlug, msisdn, ipAddress string, throttles map[string]interface{}) (bool, error) {
	// Check per-MSSDN limit
	if msisdnLimit, ok := throttles["per_msisdn_per_day"].(float64); ok && msisdnLimit > 0 {
		query := `
			SELECT COUNT(*) 
			FROM acquisition_transactions
			WHERE campaign_slug = $1 AND msisdn = $2 
			  AND status NOT IN ('FAILED', 'CANCELLED')
			  AND created_at >= CURRENT_DATE
		`
		var count int
		err := r.db.QueryRow(query, campaignSlug, msisdn).Scan(&count)
		if err != nil {
			return false, fmt.Errorf("failed to check MSISDN throttle: %w", err)
		}
		if count >= int(msisdnLimit) {
			return true, nil
		}
	}

	// Check per-IP limit
	if ipLimit, ok := throttles["per_ip_per_day"].(float64); ok && ipLimit > 0 && ipAddress != "" {
		query := `
			SELECT COUNT(*) 
			FROM acquisition_transactions
			WHERE campaign_slug = $1 AND ip_address = $2 
			  AND status NOT IN ('FAILED', 'CANCELLED')
			  AND created_at >= CURRENT_DATE
		`
		var count int
		err := r.db.QueryRow(query, campaignSlug, ipAddress).Scan(&count)
		if err != nil {
			return false, fmt.Errorf("failed to check IP throttle: %w", err)
		}
		if count >= int(ipLimit) {
			return true, nil
		}
	}

	return false, nil
}

func (r *TransactionRepository) CheckThrottleForTenant(tenantID, campaignSlug, msisdn, ipAddress string, throttles map[string]interface{}) (bool, error) {
	if strings.TrimSpace(tenantID) == "" {
		return r.CheckThrottle(campaignSlug, msisdn, ipAddress, throttles)
	}

	if msisdnLimit, ok := throttles["per_msisdn_per_day"].(float64); ok && msisdnLimit > 0 {
		query := `
			SELECT COUNT(*)
			FROM acquisition_transactions
			WHERE tenant_id = $1 AND campaign_slug = $2 AND msisdn = $3
			  AND status NOT IN ('FAILED', 'CANCELLED')
			  AND created_at >= CURRENT_DATE
		`
		var count int
		err := r.db.QueryRow(query, tenantID, campaignSlug, msisdn).Scan(&count)
		if err != nil {
			return false, fmt.Errorf("failed to check tenant MSISDN throttle: %w", err)
		}
		if count >= int(msisdnLimit) {
			return true, nil
		}
	}

	if ipLimit, ok := throttles["per_ip_per_day"].(float64); ok && ipLimit > 0 && ipAddress != "" {
		query := `
			SELECT COUNT(*)
			FROM acquisition_transactions
			WHERE tenant_id = $1 AND campaign_slug = $2 AND ip_address = $3
			  AND status NOT IN ('FAILED', 'CANCELLED')
			  AND created_at >= CURRENT_DATE
		`
		var count int
		err := r.db.QueryRow(query, tenantID, campaignSlug, ipAddress).Scan(&count)
		if err != nil {
			return false, fmt.Errorf("failed to check tenant IP throttle: %w", err)
		}
		if count >= int(ipLimit) {
			return true, nil
		}
	}

	return false, nil
}

// FindByClickID finds transactions by click ID (for idempotency)
func (r *TransactionRepository) FindByClickID(provider, clickID string) (*domain.AcquisitionTransaction, error) {
	query := `
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE ad_provider = $1 AND click_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx, err := r.scanTransaction(query, provider, clickID)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *TransactionRepository) FindByTenantClickID(tenantID, provider, clickID string) (*domain.AcquisitionTransaction, error) {
	if strings.TrimSpace(tenantID) == "" {
		return r.FindByClickID(provider, clickID)
	}
	query := `
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE tenant_id = $1 AND ad_provider = $2 AND click_id = $3
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx, err := r.scanTransaction(query, tenantID, provider, clickID)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// FindByTimweTransactionID finds a transaction by TIMWE transaction ID
func (r *TransactionRepository) FindByTimweTransactionID(timweTransactionID string) (*domain.AcquisitionTransaction, error) {
	// Check for duplicate rows sharing the same timwe_transaction_id
	var count int
	countErr := r.db.QueryRow(
		`SELECT COUNT(*) FROM acquisition_transactions WHERE timwe_transaction_id = $1`,
		timweTransactionID,
	).Scan(&count)
	if countErr == nil && count > 1 {
		r.logger.Warn("duplicate timwe_transaction_id detected, returning newest",
			zap.String("timwe_transaction_id", timweTransactionID),
			zap.Int("count", count),
		)
	}

	query := `
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE timwe_transaction_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx, err := r.scanTransaction(query, timweTransactionID)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// FindByMSISDNAndStatus finds a transaction by MSISDN and status
func (r *TransactionRepository) FindByMSISDNAndStatus(msisdn string, status domain.TransactionStatus) (*domain.AcquisitionTransaction, error) {
	query := `
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE msisdn = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx, err := r.scanTransaction(query, msisdn, status)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// FindByMSISDNAndStatuses finds the latest transaction by MSISDN matching any of the given statuses
func (r *TransactionRepository) FindByMSISDNAndStatuses(msisdn string, statuses []domain.TransactionStatus) (*domain.AcquisitionTransaction, error) {
	if len(statuses) == 0 {
		return nil, fmt.Errorf("statuses are required")
	}

	placeholders := make([]string, len(statuses))
	args := make([]interface{}, 0, len(statuses)+1)
	args = append(args, msisdn)

	for i, status := range statuses {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, string(status))
	}

	query := fmt.Sprintf(`
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE msisdn = $1 AND status IN (%s)
		ORDER BY created_at DESC
		LIMIT 1
	`, strings.Join(placeholders, ", "))

	return r.scanTransaction(query, args...)
}

// FindLatestByCampaignAndMSISDN finds the latest recent transaction for campaign+msisdn across the provided statuses.
func (r *TransactionRepository) FindLatestByCampaignAndMSISDN(campaignSlug, msisdn string, statuses []domain.TransactionStatus, notOlderThan time.Time) (*domain.AcquisitionTransaction, error) {
	if len(statuses) == 0 {
		return nil, fmt.Errorf("statuses are required")
	}

	placeholders := make([]string, len(statuses))
	args := make([]interface{}, 0, len(statuses)+3)
	args = append(args, campaignSlug, msisdn)

	for i, status := range statuses {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, string(status))
	}
	cutoffPlaceholder := fmt.Sprintf("$%d", len(statuses)+3)
	args = append(args, notOlderThan)

	query := fmt.Sprintf(`
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE campaign_slug = $1 AND msisdn = $2
		  AND status IN (%s)
		  AND created_at >= %s
		ORDER BY created_at DESC
		LIMIT 1
	`, strings.Join(placeholders, ", "), cutoffPlaceholder)

	tx, err := r.scanTransaction(query, args...)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *TransactionRepository) FindLatestByTenantCampaignAndMSISDN(tenantID, campaignSlug, msisdn string, statuses []domain.TransactionStatus, notOlderThan time.Time) (*domain.AcquisitionTransaction, error) {
	if strings.TrimSpace(tenantID) == "" {
		return r.FindLatestByCampaignAndMSISDN(campaignSlug, msisdn, statuses, notOlderThan)
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("statuses are required")
	}

	placeholders := make([]string, len(statuses))
	args := make([]interface{}, 0, len(statuses)+4)
	args = append(args, tenantID, campaignSlug, msisdn)

	for i, status := range statuses {
		placeholders[i] = fmt.Sprintf("$%d", i+4)
		args = append(args, string(status))
	}
	cutoffPlaceholder := fmt.Sprintf("$%d", len(statuses)+4)
	args = append(args, notOlderThan)

	query := fmt.Sprintf(`
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE tenant_id = $1 AND campaign_slug = $2 AND msisdn = $3
		  AND status IN (%s)
		  AND created_at >= %s
		ORDER BY created_at DESC
		LIMIT 1
	`, strings.Join(placeholders, ", "), cutoffPlaceholder)

	tx, err := r.scanTransaction(query, args...)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// MarkCharged updates a transaction to CHARGED status with charge details
func (r *TransactionRepository) MarkCharged(id uuid.UUID, chargedAt *time.Time, payout string) error {
	query := `
		UPDATE acquisition_transactions
		SET status = $1, charged_at = $2, charge_payout = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
	`

	var chargedAtVal sql.NullTime
	if chargedAt != nil {
		chargedAtVal.Time = *chargedAt
		chargedAtVal.Valid = true
	}

	var payoutVal sql.NullString
	if payout != "" {
		payoutVal.String = payout
		payoutVal.Valid = true
	}

	_, err := r.db.Exec(query, domain.StatusCharged, chargedAtVal, payoutVal, id)
	if err != nil {
		return fmt.Errorf("failed to mark transaction as charged: %w", err)
	}

	return nil
}

// MarkConversionPostbackSent marks the conversion postback as sent (idempotency)
func (r *TransactionRepository) MarkConversionPostbackSent(id uuid.UUID) error {
	query := `
		UPDATE acquisition_transactions
		SET conversion_postback_sent = true, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark conversion postback sent: %w", err)
	}

	return nil
}

// TransactionListFilter represents filters for listing transactions
type TransactionListFilter struct {
	CampaignSlug string
	Status       string
	Provider     string
	StartDate    *time.Time
	EndDate      *time.Time
	Limit        int
	Offset       int
}

// TransactionListResult represents a paginated list of transactions
type TransactionListResult struct {
	Transactions []*domain.AcquisitionTransaction
	TotalCount   int
}

// ListTransactions retrieves a paginated list of transactions with optional filters
func (r *TransactionRepository) ListTransactions(filter *TransactionListFilter) (*TransactionListResult, error) {
	// Build WHERE clause dynamically
	conditions := []string{"1=1"}
	args := []interface{}{}
	argIndex := 1

	if filter.CampaignSlug != "" {
		conditions = append(conditions, fmt.Sprintf("campaign_slug = $%d", argIndex))
		args = append(args, filter.CampaignSlug)
		argIndex++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}
	if filter.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("ad_provider = $%d", argIndex))
		args = append(args, filter.Provider)
		argIndex++
	}
	if filter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}
	if filter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	whereClause := ""
	for i, cond := range conditions {
		if i == 0 {
			whereClause = "WHERE " + cond
		} else {
			whereClause += " AND " + cond
		}
	}

	// Count query
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM acquisition_transactions %s`, whereClause)
	var totalCount int
	err := r.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Apply defaults for pagination
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Data query with pagination
	dataQuery := fmt.Sprintf(`
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       offer_product_id, pricepoint_id, partner_role_id,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       he_source, he_msisdn, he_operator,
		       charged_at, charge_payout, conversion_postback_sent,
		       created_at, updated_at
		FROM acquisition_transactions
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*domain.AcquisitionTransaction
	for rows.Next() {
		tx, err := r.scanTransactionFromRow(rows)
		if err != nil {
			r.logger.Error("Failed to scan transaction row", zap.Error(err))
			continue
		}
		transactions = append(transactions, tx)
	}

	return &TransactionListResult{
		Transactions: transactions,
		TotalCount:   totalCount,
	}, nil
}

// scanTransactionFromRow scans a single transaction from sql.Rows
func (r *TransactionRepository) scanTransactionFromRow(rows *sql.Rows) (*domain.AcquisitionTransaction, error) {
	var tx domain.AcquisitionTransaction
	var nextAction, adProvider, clickID, ipAddress, userAgent, consentVersion,
		landingVersionHash, timweTransactionID,
		transactionAuthCode, timweStatus, heSource, heMSISDN, heOperator, chargePayout sql.NullString
	var offerProductID, pricepointID, partnerRoleID sql.NullInt64
	var consentTimestamp, chargedAt sql.NullTime
	var nextActionPayload, attributionData sql.NullString

	err := rows.Scan(
		&tx.ID, &tx.CorrelationID, &tx.CampaignSlug, &tx.MSISDN, &tx.Status,
		&nextAction, &nextActionPayload, &adProvider, &clickID, &attributionData,
		&ipAddress, &userAgent, &tx.ConsentRequired, &tx.ConsentChecked,
		&consentVersion, &consentTimestamp, &landingVersionHash,
		&offerProductID, &pricepointID, &partnerRoleID,
		&timweTransactionID, &transactionAuthCode, &timweStatus,
		&heSource, &heMSISDN, &heOperator,
		&chargedAt, &chargePayout, &tx.ConversionPostbackSent,
		&tx.CreatedAt, &tx.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan transaction: %w", err)
	}

	// Map nullable fields
	if nextAction.Valid {
		na := domain.NextAction(nextAction.String)
		tx.NextAction = &na
	}
	if adProvider.Valid {
		tx.AdProvider = &adProvider.String
	}
	if clickID.Valid {
		tx.ClickID = &clickID.String
	}
	if ipAddress.Valid {
		tx.IPAddress = &ipAddress.String
	}
	if userAgent.Valid {
		tx.UserAgent = &userAgent.String
	}
	if consentVersion.Valid {
		tx.ConsentVersion = &consentVersion.String
	}
	if consentTimestamp.Valid {
		tx.ConsentTimestamp = &consentTimestamp.Time
	}
	if landingVersionHash.Valid {
		tx.LandingVersionHash = &landingVersionHash.String
	}
	if offerProductID.Valid {
		val := int(offerProductID.Int64)
		tx.OfferProductID = &val
	}
	if pricepointID.Valid {
		val := int(pricepointID.Int64)
		tx.PricepointID = &val
	}
	if partnerRoleID.Valid {
		val := int(partnerRoleID.Int64)
		tx.PartnerRoleID = &val
	}
	if timweTransactionID.Valid {
		tx.TimweTransactionID = &timweTransactionID.String
	}
	if transactionAuthCode.Valid {
		tx.TransactionAuthCode = &transactionAuthCode.String
	}
	if timweStatus.Valid {
		tx.TimweStatus = &timweStatus.String
	}
	if heSource.Valid {
		src := domain.HESource(heSource.String)
		tx.HESource = &src
	}
	if heMSISDN.Valid {
		tx.HEMSISDN = &heMSISDN.String
	}
	if heOperator.Valid {
		tx.HEOperator = &heOperator.String
	}
	if chargedAt.Valid {
		tx.ChargedAt = &chargedAt.Time
	}
	if chargePayout.Valid {
		tx.ChargePayout = &chargePayout.String
	}
	if nextActionPayload.Valid {
		tx.NextActionPayload = json.RawMessage(nextActionPayload.String)
	}
	if attributionData.Valid {
		tx.AttributionData = json.RawMessage(attributionData.String)
	}

	return &tx, nil
}
