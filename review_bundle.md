# Review Bundle

- Generated: `2026-01-26 02:41:45Z`

- Branch: `main`

- Base ref: `origin/main` (merge-base `b86522933b13108dd7165f0f91618a59c378d5bc`)

- HEAD: `0a272eb51397fb605a10fe4bceed173c1f624e6f`


## Files changed (name-status)

```text
A	services/acquisition-api/internal/domain/transaction.go
A	services/acquisition-api/internal/handler/he_context.go
A	services/acquisition-api/internal/repository/transaction_repository.go
A	services/acquisition-api/internal/service/transaction_service.go
A	services/billing/go.mod
A	services/billing/go.sum
A	services/billing/internal/handler/health.go
A	services/billing/internal/handler/http.go
A	services/billing/internal/handler/metrics.go
A	services/billing/internal/repository/postgres.go
A	services/billing/internal/service/billing.go
A	services/billing/internal/service/billing_test.go
A	services/subscription-external/migrations/013_he_tracking.sql
```


## Diff stat

```text
.../acquisition-api/internal/domain/transaction.go | 150 +++++
 .../acquisition-api/internal/handler/he_context.go | 281 +++++++++
 .../internal/repository/transaction_repository.go  | 662 +++++++++++++++++++++
 .../internal/service/transaction_service.go        | 551 +++++++++++++++++
 services/billing/go.mod                            |  48 ++
 services/billing/go.sum                            | 104 ++++
 services/billing/internal/handler/health.go        |  18 +
 services/billing/internal/handler/http.go          |  92 +++
 services/billing/internal/handler/metrics.go       |  94 +++
 services/billing/internal/repository/postgres.go   |  53 ++
 services/billing/internal/service/billing.go       |  36 ++
 services/billing/internal/service/billing_test.go  |  38 ++
 .../migrations/013_he_tracking.sql                 |  18 +
 13 files changed, 2145 insertions(+)
```


## Unified diff (truncated)

```diff
diff --git a/services/acquisition-api/internal/domain/transaction.go b/services/acquisition-api/internal/domain/transaction.go
new file mode 100644
index 0000000..8121a8c
--- /dev/null
+++ b/services/acquisition-api/internal/domain/transaction.go
@@ -0,0 +1,150 @@
+package domain
+
+import (
+	"encoding/json"
+	"time"
+
+	"github.com/google/uuid"
+)
+
+// TransactionStatus represents the status of an acquisition transaction
+type TransactionStatus string
+
+const (
+	StatusPending         TransactionStatus = "PENDING"
+	StatusActionRequired  TransactionStatus = "ACTION_REQUIRED"
+	StatusConfirmRequired TransactionStatus = "CONFIRM_REQUIRED"
+	StatusSubscribed      TransactionStatus = "SUBSCRIBED"
+	StatusCharged         TransactionStatus = "CHARGED" // Charge success confirmed by subscription-external
+	StatusFailed          TransactionStatus = "FAILED"
+	StatusCancelled       TransactionStatus = "CANCELLED"
+)
+
+// NextAction represents the next action the user should take
+type NextAction string
+
+const (
+	NextActionOpenSMS          NextAction = "OPEN_SMS"
+	NextActionOTP              NextAction = "OTP"
+	NextActionRedirect         NextAction = "REDIRECT"
+	NextActionShowInstructions NextAction = "SHOW_INSTRUCTIONS"
+	NextActionSubscribed       NextAction = "SUBSCRIBED" // HE path - direct subscription
+)
+
+// HESource represents the source of Header Enrichment identity
+type HESource string
+
+const (
+	HESourceReal      HESource = "REAL"      // Real HE headers from MNO
+	HESourceSimulated HESource = "SIMULATED" // Simulated for testing
+	HESourceNone      HESource = "NONE"      // No HE detected
+)
+
+// AcquisitionTransaction represents a web acquisition attempt
+type AcquisitionTransaction struct {
+	ID            uuid.UUID          `json:"id" db:"id"`
+	CorrelationID uuid.UUID          `json:"correlation_id" db:"correlation_id"`
+	
+	// Campaign and user
+	CampaignSlug string              `json:"campaign_slug" db:"campaign_slug"`
+	MSISDN       string              `json:"msisdn" db:"msisdn"`
+	
+	// Status and flow
+	Status           TransactionStatus `json:"status" db:"status"`
+	NextAction       *NextAction       `json:"next_action,omitempty" db:"next_action"`
+	NextActionPayload json.RawMessage   `json:"next_action_payload,omitempty" db:"next_action_payload"`
+	
+	// Attribution
+	AdProvider      *string           `json:"ad_provider,omitempty" db:"ad_provider"`
+	ClickID         *string           `json:"click_id,omitempty" db:"click_id"`
+	AttributionData json.RawMessage   `json:"attribution_data" db:"attribution_data"`
+	
+	// Request metadata
+	IPAddress *string                 `json:"ip_address,omitempty" db:"ip_address"`
+	UserAgent *string                 `json:"user_agent,omitempty" db:"user_agent"`
+	
+	// Consent tracking
+	ConsentRequired    bool            `json:"consent_required" db:"consent_required"`
+	ConsentChecked     bool            `json:"consent_checked" db:"consent_checked"`
+	ConsentVersion     *string         `json:"consent_version,omitempty" db:"consent_version"`
+	ConsentTimestamp   *time.Time      `json:"consent_timestamp,omitempty" db:"consent_timestamp"`
+	LandingVersionHash *string         `json:"landing_version_hash,omitempty" db:"landing_version_hash"`
+	
+	// Header Enrichment (HE) tracking
+	HESource   *HESource `json:"he_source,omitempty" db:"he_source"`
+	HEMSISDN   *string   `json:"he_msisdn,omitempty" db:"he_msisdn"`
+	HEOperator *string   `json:"he_operator,omitempty" db:"he_operator"`
+
+	// TIMWE integration
+	TimweTransactionID  *string `json:"timwe_transaction_id,omitempty" db:"timwe_transaction_id"`
+	TransactionAuthCode *string `json:"transaction_auth_code,omitempty" db:"transaction_auth_code"`
+	TimweStatus         *string `json:"timwe_status,omitempty" db:"timwe_status"`
+
+	// Charge tracking (for conversion postbacks)
+	ChargedAt              *time.Time `json:"charged_at,omitempty" db:"charged_at"`
+	ChargePayout           *string    `json:"charge_payout,omitempty" db:"charge_payout"`
+	ConversionPostbackSent bool       `json:"conversion_postback_sent" db:"conversion_postback_sent"`
+
+	// Timestamps
+	CreatedAt time.Time `json:"created_at" db:"created_at"`
+	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
+}
+
+// CreateTransactionRequest represents the request to create a new transaction
+type CreateTransactionRequest struct {
+	CampaignSlug string `json:"campaign_slug" binding:"required"`
+	MSISDN       string `json:"msisdn" binding:"required"`
+
+	// Attribution (will be normalized by provider)
+	Provider        *string           `json:"provider,omitempty"`
+	ClickID         *string           `json:"click_id,omitempty"`
+	AttributionData map[string]string `json:"attribution_data,omitempty"`
+
+	// Consent
+	ConsentChecked bool `json:"consent_checked"`
+
+	// Request metadata (optional, can be extracted from headers)
+	IPAddress *string `json:"ip_address,omitempty"`
+	UserAgent *string `json:"user_agent,omitempty"`
+
+	// Header Enrichment context (populated from middleware)
+	HESource   *HESource `json:"-"` // Not from JSON, set by handler
+	HEMSISDN   *string   `json:"-"` // Not from JSON, set by handler
+	HEOperator *string   `json:"-"` // Not from JSON, set by handler
+}
+
+// CreateTransactionResponse represents the response after creating a transaction
+type CreateTransactionResponse struct {
+	TransactionID uuid.UUID            `json:"transaction_id"`
+	CorrelationID uuid.UUID            `json:"correlation_id"`
+	Status        TransactionStatus     `json:"status"`
+	NextAction    NextAction            `json:"next_action"`
+	Payload       map[string]interface{} `json:"payload"`
+}
+
+// ConfirmTransactionRequest represents the request to confirm a transaction (OTP flow)
+type ConfirmTransactionRequest struct {
+	TransactionID uuid.UUID            `json:"transaction_id" binding:"required"`
+	AuthCode      string                `json:"auth_code" binding:"required"`
+}
+
+// TransactionStatusResponse represents the current status of a transaction
+type TransactionStatusResponse struct {
+	TransactionID uuid.UUID            `json:"transaction_id"`
+	Status        TransactionStatus     `json:"status"`
+	NextAction    *NextAction          `json:"next_action,omitempty"`
+	Payload       map[string]interface{} `json:"payload,omitempty"`
+}
+
+// Attribution represents normalized attribution data
+type Attribution struct {
+	Provider      string
+	ClickID       string
+	PubID         string
+	Sub1          string
+	Sub2          string
+	Sub3          string
+	CampaignSlug  string
+	Creative      string
+	Source        string
+}
diff --git a/services/acquisition-api/internal/handler/he_context.go b/services/acquisition-api/internal/handler/he_context.go
new file mode 100644
index 0000000..3023fed
--- /dev/null
+++ b/services/acquisition-api/internal/handler/he_context.go
@@ -0,0 +1,281 @@
+package handler
+
+import (
+	"crypto/sha256"
+	"encoding/hex"
+	"regexp"
+	"strings"
+
+	"github.com/valyala/fasthttp"
+	"go.uber.org/zap"
+)
+
+// HESource represents the source of Header Enrichment identity
+type HESource string
+
+const (
+	HESourceReal      HESource = "REAL"
+	HESourceSimulated HESource = "SIMULATED"
+	HESourceNone      HESource = "NONE"
+)
+
+// HEIdentity represents the resolved HE identity from headers
+type HEIdentity struct {
+	MSISDN     string
+	OperatorID string
+	MCC        string
+	MNC        string
+	Source     HESource
+}
+
+// HEContextConfig holds configuration for HE detection
+type HEContextConfig struct {
+	SimulationEnabled bool
+	MSISDNHeaders     []string
+	MCCHeader         string
+	MNCHeader         string
+	OperatorHeader    string
+}
+
+// DefaultHEContextConfig returns the default HE context configuration
+func DefaultHEContextConfig() *HEContextConfig {
+	return &HEContextConfig{
+		SimulationEnabled: false,
+		MSISDNHeaders: []string{
+			"X-MSISDN",
+			"X-UP-CALLING-LINE-ID",
+			"X_WAP_NETWORK_CLIENT_MSISDN",
+		},
+		MCCHeader:      "X-MCC",
+		MNCHeader:      "X-MNC",
+		OperatorHeader: "X-Operator-ID",
+	}
+}
+
+// HEContextKey is the context key for HE identity
+type HEContextKey struct{}
+
+// Request context key for storing HE identity
+const heContextKeyName = "he_identity"
+
+// Headers passed from frontend (simulation flow)
+const (
+	HeaderHESource   = "X-He-Source"
+	HeaderHEMSISDN   = "X-He-Msisdn"
+	HeaderHEOperator = "X-He-Operator"
+	HeaderHEMCC      = "X-He-Mcc"
+	HeaderHEMNC      = "X-He-Mnc"
+)
+
+// HEContextMiddleware creates middleware for extracting HE identity
+type HEContextMiddleware struct {
+	config *HEContextConfig
+	logger *zap.Logger
+}
+
+// NewHEContextMiddleware creates a new HE context middleware
+func NewHEContextMiddleware(config *HEContextConfig, logger *zap.Logger) *HEContextMiddleware {
+	if config == nil {
+		config = DefaultHEContextConfig()
+	}
+	return &HEContextMiddleware{
+		config: config,
+		logger: logger,
+	}
+}
+
+// ExtractIdentity extracts HE identity from the request
+// Priority: Real HE headers > Simulated (from frontend) > None
+func (m *HEContextMiddleware) ExtractIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
+	// 1. Try real HE headers first (from MNO proxy)
+	identity := m.extractRealHEIdentity(ctx)
+	if identity != nil {
+		// Enrich with operator info from MSISDN prefix if not in headers
+		EnrichIdentityWithOperator(identity)
+		m.logIdentity("Real HE identity detected", identity)
+		return identity
+	}
+
+	// 2. Try simulated identity (passed from frontend via headers)
+	if m.config.SimulationEnabled {
+		identity = m.extractSimulatedIdentity(ctx)
+		if identity != nil {
+			// Enrich with operator info from MSISDN prefix if not provided
+			EnrichIdentityWithOperator(identity)
+			m.logIdentity("Simulated HE identity detected", identity)
+			return identity
+		}
+	}
+
+	return nil
+}
+
+// extractRealHEIdentity extracts identity from real MNO HE headers
+func (m *HEContextMiddleware) extractRealHEIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
+	var msisdn string
+
+	// Check candidate MSISDN headers in order of preference
+	for _, headerName := range m.config.MSISDNHeaders {
+		value := string(ctx.Request.Header.Peek(headerName))
+		if value != "" {
+			normalized := normalizeMSISDN(value)
+			if isValidMSISDN(normalized) {
+				msisdn = normalized
+				break
+			}
+		}
+	}
+
+	if msisdn == "" {
+		return nil
+	}
+
+	return &HEIdentity{
+		MSISDN:     msisdn,
+		OperatorID: string(ctx.Request.Header.Peek(m.config.OperatorHeader)),
+		MCC:        string(ctx.Request.Header.Peek(m.config.MCCHeader)),
+		MNC:        string(ctx.Request.Header.Peek(m.config.MNCHeader)),
+		Source:     HESourceReal,
+	}
+}
+
+// extractSimulatedIdentity extracts identity from simulation headers (set by frontend)
+func (m *HEContextMiddleware) extractSimulatedIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
+	source := string(ctx.Request.Header.Peek(HeaderHESource))
+	msisdn := string(ctx.Request.Header.Peek(HeaderHEMSISDN))
+
+	// Only accept if source indicates simulation and MSISDN is valid
+	if source != string(HESourceSimulated) || msisdn == "" {
+		return nil
+	}
+
+	normalized := normalizeMSISDN(msisdn)
+	if !isValidMSISDN(normalized) {
+		return nil
+	}
+
+	return &HEIdentity{
+		MSISDN:     normalized,
+		OperatorID: string(ctx.Request.Header.Peek(HeaderHEOperator)),
+		MCC:        string(ctx.Request.Header.Peek(HeaderHEMCC)),
+		MNC:        string(ctx.Request.Header.Peek(HeaderHEMNC)),
+		Source:     HESourceSimulated,
+	}
+}
+
+// logIdentity logs the detected identity (with MSISDN hashed for privacy)
+func (m *HEContextMiddleware) logIdentity(msg string, identity *HEIdentity) {
+	m.logger.Info(msg,
+		zap.String("he_source", string(identity.Source)),
+		zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
+		zap.String("operator_id", identity.OperatorID),
+		zap.String("mcc", identity.MCC),
+		zap.String("mnc", identity.MNC),
+	)
+}
+
+// normalizeMSISDN removes whitespace and leading '+' from MSISDN
+func normalizeMSISDN(msisdn string) string {
+	// Remove all whitespace
+	msisdn = strings.ReplaceAll(msisdn, " ", "")
+	msisdn = strings.ReplaceAll(msisdn, "\t", "")
+	// Remove leading '+'
+	msisdn = strings.TrimPrefix(msisdn, "+")
+	return msisdn
+}
+
+// isValidMSISDN validates MSISDN format (9-15 digits)
+func isValidMSISDN(msisdn string) bool {
+	matched, _ := regexp.MatchString(`^\d{9,15}$`, msisdn)
+	return matched
+}
+
+// hashMSISDN returns SHA256 hash of MSISDN for logging
+func hashMSISDN(msisdn string) string {
+	hash := sha256.Sum256([]byte(msisdn))
+	return hex.EncodeToString(hash[:8]) // First 8 bytes for brevity
+}
+
+// GhanaOperator represents a Ghana MNO with prefix mappings
+// Based on docs/ghana-header-enrichment-parameters.md
+type GhanaOperator struct {
+	Name     string
+	MCC      string
+	MNC      string
+	Prefixes []string
+}
+
+// GhanaOperators contains the Ghana MNO configurations
+// MCC 620 for all Ghana operators
+var GhanaOperators = []GhanaOperator{
+	{
+		Name:     "MTN Ghana",
+		MCC:      "620",
+		MNC:      "01",
+		Prefixes: []string{"23324", "23354", "23355", "23353"},
+	},
+	{
+		Name:     "Telecel Ghana",
+		MCC:      "620",
+		MNC:      "02",
+		Prefixes: []string{"23320", "23350"},
+	},
+	{
+		Name:     "AT Ghana",
+		MCC:      "620",
+		MNC:      "03",
+		Prefixes: []string{"23326", "23327", "23356", "23357"},
+	},
+}
+
+// DetectGhanaOperator detects the operator from MSISDN prefix
+// Returns nil if no matching operator found
+func DetectGhanaOperator(msisdn string) *GhanaOperator {
+	normalized := normalizeMSISDN(msisdn)
+	for i := range GhanaOperators {
+		for _, prefix := range GhanaOperators[i].Prefixes {
+			if strings.HasPrefix(normalized, prefix) {
+				return &GhanaOperators[i]
+			}
+		}
+	}
+	return nil
+}
+
+// EnrichIdentityWithOperator adds operator info to identity based on MSISDN prefix
+// if operator info is not already present from headers
+func EnrichIdentityWithOperator(identity *HEIdentity) {
+	if identity == nil {
+		return
+	}
+
+	// Only enrich if operator not already set from headers
+	if identity.OperatorID == "" || identity.MCC == "" || identity.MNC == "" {
+		if op := DetectGhanaOperator(identity.MSISDN); op != nil {
+			if identity.OperatorID == "" {
+				identity.OperatorID = op.Name
+			}
+			if identity.MCC == "" {
+				identity.MCC = op.MCC
+			}
+			if identity.MNC == "" {
+				identity.MNC = op.MNC
+			}
+		}
+	}
+}
+
+// StoreIdentity stores HE identity in the request context
+func StoreIdentity(ctx *fasthttp.RequestCtx, identity *HEIdentity) {
+	ctx.SetUserValue(heContextKeyName, identity)
+}
+
+// GetIdentity retrieves HE identity from the request context
+func GetIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
+	if val := ctx.UserValue(heContextKeyName); val != nil {
+		if identity, ok := val.(*HEIdentity); ok {
+			return identity
+		}
+	}
+	return nil
+}
diff --git a/services/acquisition-api/internal/repository/transaction_repository.go b/services/acquisition-api/internal/repository/transaction_repository.go
new file mode 100644
index 0000000..a322179
--- /dev/null
+++ b/services/acquisition-api/internal/repository/transaction_repository.go
@@ -0,0 +1,662 @@
+package repository
+
+import (
+	"database/sql"
+	"encoding/json"
+	"fmt"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
+	"go.uber.org/zap"
+)
+
+// TransactionRepository handles acquisition transaction data access
+type TransactionRepository struct {
+	db     *sql.DB
+	logger *zap.Logger
+}
+
+// NewTransactionRepository creates a new transaction repository
+func NewTransactionRepository(db *sql.DB, logger *zap.Logger) *TransactionRepository {
+	return &TransactionRepository{
+		db:     db,
+		logger: logger,
+	}
+}
+
+// DB returns the underlying database connection for advanced queries
+func (r *TransactionRepository) DB() *sql.DB {
+	return r.db
+}
+
+// Create creates a new acquisition transaction
+func (r *TransactionRepository) Create(tx *domain.AcquisitionTransaction) error {
+	query := `
+		INSERT INTO acquisition_transactions (
+			id, correlation_id, campaign_slug, msisdn, status, next_action,
+			next_action_payload, ad_provider, click_id, attribution_data,
+			ip_address, user_agent, consent_required, consent_checked,
+			consent_version, consent_timestamp, landing_version_hash,
+			timwe_transaction_id, transaction_auth_code, timwe_status,
+			he_source, he_msisdn, he_operator,
+			created_at, updated_at
+		) VALUES (
+			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
+			$15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25
+		)
+	`
+
+	var nextAction, adProvider, clickID, ipAddress, userAgent, consentVersion,
+		landingVersionHash, timweTransactionID,
+		transactionAuthCode, timweStatus, heSource, heMSISDN, heOperator sql.NullString
+	var consentTimestamp sql.NullTime
+	var nextActionPayload sql.NullString
+	var attributionData sql.NullString
+
+	if tx.NextAction != nil {
+		nextAction.String = string(*tx.NextAction)
+		nextAction.Valid = true
+	}
+	if tx.AdProvider != nil {
+		adProvider.String = *tx.AdProvider
+		adProvider.Valid = true
+	}
+	if tx.ClickID != nil {
+		clickID.String = *tx.ClickID
+		clickID.Valid = true
+	}
+	if tx.IPAddress != nil {
+		ipAddress.String = *tx.IPAddress
+		ipAddress.Valid = true
+	}
+	if tx.UserAgent != nil {
+		userAgent.String = *tx.UserAgent
+		userAgent.Valid = true
+	}
+	if tx.ConsentVersion != nil {
+		consentVersion.String = *tx.ConsentVersion
+		consentVersion.Valid = true
+	}
+	if tx.ConsentTimestamp != nil {
+		consentTimestamp.Time = *tx.ConsentTimestamp
+		consentTimestamp.Valid = true
+	}
+	if tx.LandingVersionHash != nil {
+		landingVersionHash.String = *tx.LandingVersionHash
+		landingVersionHash.Valid = true
+	}
+	if tx.TimweTransactionID != nil {
+		timweTransactionID.String = *tx.TimweTransactionID
+		timweTransactionID.Valid = true
+	}
+	if tx.TransactionAuthCode != nil {
+		transactionAuthCode.String = *tx.TransactionAuthCode
+		transactionAuthCode.Valid = true
+	}
+	if tx.TimweStatus != nil {
+		timweStatus.String = *tx.TimweStatus
+		timweStatus.Valid = true
+	}
+	if tx.HESource != nil {
+		heSource.String = string(*tx.HESource)
+		heSource.Valid = true
+	}
+	if tx.HEMSISDN != nil {
+		heMSISDN.String = *tx.HEMSISDN
+		heMSISDN.Valid = true
+	}
+	if tx.HEOperator != nil {
+		heOperator.String = *tx.HEOperator
+		heOperator.Valid = true
+	}
+
+	if len(tx.NextActionPayload) > 0 {
+		nextActionPayload.String = string(tx.NextActionPayload)
+		nextActionPayload.Valid = true
+	}
+	if len(tx.AttributionData) > 0 {
+		attributionData.String = string(tx.AttributionData)
+		attributionData.Valid = true
+	}
+
+	_, err := r.db.Exec(query,
+		tx.ID, tx.CorrelationID, tx.CampaignSlug, tx.MSISDN, tx.Status,
+		nextAction, nextActionPayload, adProvider, clickID, attributionData,
+		ipAddress, userAgent, tx.ConsentRequired, tx.ConsentChecked,
+		consentVersion, consentTimestamp, landingVersionHash,
+		timweTransactionID, transactionAuthCode, timweStatus,
+		heSource, heMSISDN, heOperator,
+		tx.CreatedAt, tx.UpdatedAt,
+	)
+
+	if err != nil {
+		return fmt.Errorf("failed to create transaction: %w", err)
+	}
+
+	return nil
+}
+
+// GetByID retrieves a transaction by ID
+func (r *TransactionRepository) GetByID(id uuid.UUID) (*domain.AcquisitionTransaction, error) {
+	query := `
+		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
+		       next_action_payload, ad_provider, click_id, attribution_data,
+		       ip_address, user_agent, consent_required, consent_checked,
+		       consent_version, consent_timestamp, landing_version_hash,
+		       timwe_transaction_id, transaction_auth_code, timwe_status,
+		       he_source, he_msisdn, he_operator,
+		       charged_at, charge_payout, conversion_postback_sent,
+		       created_at, updated_at
+		FROM acquisition_transactions
+		WHERE id = $1
+	`
+
+	tx, err := r.scanTransaction(query, id)
+	if err != nil {
+		return nil, err
+	}
+
+	return tx, nil
+}
+
+// UpdateStatus updates the transaction status and related fields
+func (r *TransactionRepository) UpdateStatus(id uuid.UUID, status domain.TransactionStatus, nextAction *domain.NextAction, payload json.RawMessage) error {
+	query := `
+		UPDATE acquisition_transactions
+		SET status = $1, next_action = $2, next_action_payload = $3, updated_at = CURRENT_TIMESTAMP
+		WHERE id = $4
+	`
+
+	var nextActionVal sql.NullString
+	if nextAction != nil {
+		nextActionVal.String = string(*nextAction)
+		nextActionVal.Valid = true
+	}
+
+	var payloadVal sql.NullString
+	if len(payload) > 0 {
+		payloadVal.String = string(payload)
+		payloadVal.Valid = true
+	}
+
+	_, err := r.db.Exec(query, status, nextActionVal, payloadVal, id)
+	if err != nil {
+		return fmt.Errorf("failed to update transaction status: %w", err)
+	}
+
+	return nil
+}
+
+// UpdateTIMWEData updates TIMWE-related fields
+func (r *TransactionRepository) UpdateTIMWEData(id uuid.UUID, transactionID, authCode, status string) error {
+	query := `
+		UPDATE acquisition_transactions
+		SET timwe_transaction_id = $1, transaction_auth_code = $2, 
+		    timwe_status = $3, updated_at = CURRENT_TIMESTAMP
+		WHERE id = $4
+	`
+
+	_, err := r.db.Exec(query, transactionID, authCode, status, id)
+	if err != nil {
+		return fmt.Errorf("failed to update TIMWE data: %w", err)
+	}
+
+	return nil
+}
+
+// ScanTransaction scans a transaction from a query (exported for use by handlers)
+func (r *TransactionRepository) ScanTransaction(query string, args ...interface{}) (*domain.AcquisitionTransaction, error) {
+	return r.scanTransaction(query, args...)
+}
+
+// scanTransaction scans a transaction from a query
+func (r *TransactionRepository) scanTransaction(query string, args ...interface{}) (*domain.AcquisitionTransaction, error) {
+	var tx domain.AcquisitionTransaction
+	var nextAction, adProvider, clickID, ipAddress, userAgent, consentVersion,
+		landingVersionHash, timweTransactionID,
+		transactionAuthCode, timweStatus, heSource, heMSISDN, heOperator, chargePayout sql.NullString
+	var consentTimestamp, chargedAt sql.NullTime
+	var nextActionPayload, attributionData sql.NullString
+
+	err := r.db.QueryRow(query, args...).Scan(
+		&tx.ID, &tx.CorrelationID, &tx.CampaignSlug, &tx.MSISDN, &tx.Status,
+		&nextAction, &nextActionPayload, &adProvider, &clickID, &attributionData,
+		&ipAddress, &userAgent, &tx.ConsentRequired, &tx.ConsentChecked,
+		&consentVersion, &consentTimestamp, &landingVersionHash,
+		&timweTransactionID, &transactionAuthCode, &timweStatus,
+		&heSource, &heMSISDN, &heOperator,
+		&chargedAt, &chargePayout, &tx.ConversionPostbackSent,
+		&tx.CreatedAt, &tx.UpdatedAt,
+	)
+
+	if err == sql.ErrNoRows {
+		return nil, fmt.Errorf("transaction not found")
+	}
+	if err != nil {
+		return nil, fmt.Errorf("failed to scan transaction: %w", err)
+	}
+
+	// Map nullable fields
+	if nextAction.Valid {
+		action := domain.NextAction(nextAction.String)
+		tx.NextAction = &action
+	}
+	if adProvider.Valid {
+		tx.AdProvider = &adProvider.String
+	}
+	if clickID.Valid {
+		tx.ClickID = &clickID.String
+	}
+	if ipAddress.Valid {
+		tx.IPAddress = &ipAddress.String
+	}
+	if userAgent.Valid {
+		tx.UserAgent = &userAgent.String
+	}
+	if consentVersion.Valid {
+		tx.ConsentVersion = &consentVersion.String
+	}
+	if consentTimestamp.Valid {
+		tx.ConsentTimestamp = &consentTimestamp.Time
+	}
+	if landingVersionHash.Valid {
+		tx.LandingVersionHash = &landingVersionHash.String
+	}
+	if timweTransactionID.Valid {
+		tx.TimweTransactionID = &timweTransactionID.String
+	}
+	if transactionAuthCode.Valid {
+		tx.TransactionAuthCode = &transactionAuthCode.String
+	}
+	if timweStatus.Valid {
+		tx.TimweStatus = &timweStatus.String
+	}
+	if heSource.Valid {
+		src := domain.HESource(heSource.String)
+		tx.HESource = &src
+	}
+	if heMSISDN.Valid {
+		tx.HEMSISDN = &heMSISDN.String
+	}
+	if heOperator.Valid {
+		tx.HEOperator = &heOperator.String
+	}
+	if chargedAt.Valid {
+		tx.ChargedAt = &chargedAt.Time
+	}
+	if chargePayout.Valid {
+		tx.ChargePayout = &chargePayout.String
+	}
+
+	if nextActionPayload.Valid {
+		tx.NextActionPayload = json.RawMessage(nextActionPayload.String)
+	}
+	if attributionData.Valid {
+		tx.AttributionData = json.RawMessage(attributionData.String)
+	}
+
+	return &tx, nil
+}
+
+// CheckThrottle checks if a request should be throttled based on campaign rules
+func (r *TransactionRepository) CheckThrottle(campaignSlug, msisdn, ipAddress string, throttles map[string]interface{}) (bool, error) {
+	// Check per-MSSDN limit
+	if msisdnLimit, ok := throttles["per_msisdn_per_day"].(float64); ok && msisdnLimit > 0 {
+		query := `
+			SELECT COUNT(*) 
+			FROM acquisition_transactions
+			WHERE campaign_slug = $1 AND msisdn = $2 
+			  AND created_at >= CURRENT_DATE
+		`
+		var count int
+		err := r.db.QueryRow(query, campaignSlug, msisdn).Scan(&count)
+		if err != nil {
+			return false, fmt.Errorf("failed to check MSISDN throttle: %w", err)
+		}
+		if count >= int(msisdnLimit) {
+			return true, nil
+		}
+	}
+
+	// Check per-IP limit
+	if ipLimit, ok := throttles["per_ip_per_day"].(float64); ok && ipLimit > 0 && ipAddress != "" {
+		query := `
+			SELECT COUNT(*) 
+			FROM acquisition_transactions
+			WHERE campaign_slug = $1 AND ip_address = $2 
+			  AND created_at >= CURRENT_DATE
+		`
+		var count int
+		err := r.db.QueryRow(query, campaignSlug, ipAddress).Scan(&count)
+		if err != nil {
+			return false, fmt.Errorf("failed to check IP throttle: %w", err)
+		}
+		if count >= int(ipLimit) {
+			return true, nil
+		}
+	}
+
+	return false, nil
+}
+
+// FindByClickID finds transactions by click ID (for idempotency)
+func (r *TransactionRepository) FindByClickID(provider, clickID string) (*domain.AcquisitionTransaction, error) {
+	query := `
+		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
+		       next_action_payload, ad_provider, click_id, attribution_data,
+		       ip_address, user_agent, consent_required, consent_checked,
+		       consent_version, consent_timestamp, landing_version_hash,
+		       timwe_transaction_id, transaction_auth_code, timwe_status,
+		       he_source, he_msisdn, he_operator,
+		       charged_at, charge_payout, conversion_postback_sent,
+		       created_at, updated_at
+		FROM acquisition_transactions
+		WHERE ad_provider = $1 AND click_id = $2
+		ORDER BY created_at DESC
+		LIMIT 1
+	`
+
+	tx, err := r.scanTransaction(query, provider, clickID)
+	if err != nil {
+		return nil, err
+	}
+
+	return tx, nil
+}
+
+// FindByTimweTransactionID finds a transaction by TIMWE transaction ID
+func (r *TransactionRepository) FindByTimweTransactionID(timweTransactionID string) (*domain.AcquisitionTransaction, error) {
+	query := `
+		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
+		       next_action_payload, ad_provider, click_id, attribution_data,
+		       ip_address, user_agent, consent_required, consent_checked,
+		       consent_version, consent_timestamp, landing_version_hash,
+		       timwe_transaction_id, transaction_auth_code, timwe_status,
+		       he_source, he_msisdn, he_operator,
+		       charged_at, charge_payout, conversion_postback_sent,
+		       created_at, updated_at
+		FROM acquisition_transactions
+		WHERE timwe_transaction_id = $1
+		ORDER BY created_at DESC
+		LIMIT 1
+	`
+
+	tx, err := r.scanTransaction(query, timweTransactionID)
+	if err != nil {
+		return nil, err
+	}
+
+	return tx, nil
+}
+
+// FindByMSISDNAndStatus finds a transaction by MSISDN and status
+func (r *TransactionRepository) FindByMSISDNAndStatus(msisdn string, status domain.TransactionStatus) (*domain.AcquisitionTransaction, error) {
+	query := `
+		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
+		       next_action_payload, ad_provider, click_id, attribution_data,
+		       ip_address, user_agent, consent_required, consent_checked,
+		       consent_version, consent_timestamp, landing_version_hash,
+		       timwe_transaction_id, transaction_auth_code, timwe_status,
+		       he_source, he_msisdn, he_operator,
+		       charged_at, charge_payout, conversion_postback_sent,
+		       created_at, updated_at
+		FROM acquisition_transactions
+		WHERE msisdn = $1 AND status = $2
+		ORDER BY created_at DESC
+		LIMIT 1
+	`
+
+	tx, err := r.scanTransaction(query, msisdn, status)
+	if err != nil {
+		return nil, err
+	}
+
+	return tx, nil
+}
+
+// MarkCharged updates a transaction to CHARGED status with charge details
+func (r *TransactionRepository) MarkCharged(id uuid.UUID, chargedAt *time.Time, payout string) error {
+	query := `
+		UPDATE acquisition_transactions
+		SET status = $1, charged_at = $2, charge_payout = $3, updated_at = CURRENT_TIMESTAMP
+		WHERE id = $4
+	`
+
+	var chargedAtVal sql.NullTime
+	if chargedAt != nil {
+		chargedAtVal.Time = *chargedAt
+		chargedAtVal.Valid = true
+	}
+
+	var payoutVal sql.NullString
+	if payout != "" {
+		payoutVal.String = payout
+		payoutVal.Valid = true
+	}
+
+	_, err := r.db.Exec(query, domain.StatusCharged, chargedAtVal, payoutVal, id)
+	if err != nil {
+		return fmt.Errorf("failed to mark transaction as charged: %w", err)
+	}
+
+	return nil
+}
+
+// MarkConversionPostbackSent marks the conversion postback as sent (idempotency)
+func (r *TransactionRepository) MarkConversionPostbackSent(id uuid.UUID) error {
+	query := `
+		UPDATE acquisition_transactions
+		SET conversion_postback_sent = true, updated_at = CURRENT_TIMESTAMP
+		WHERE id = $1
+	`
+
+	_, err := r.db.Exec(query, id)
+	if err != nil {
+		return fmt.Errorf("failed to mark conversion postback sent: %w", err)
+	}
+
+	return nil
+}
+
+// TransactionListFilter represents filters for listing transactions
+type TransactionListFilter struct {
+	CampaignSlug string
+	Status       string
+	Provider     string
+	StartDate    *time.Time
+	EndDate      *time.Time
+	Limit        int
+	Offset       int
+}
+
+// TransactionListResult represents a paginated list of transactions
+type TransactionListResult struct {
+	Transactions []*domain.AcquisitionTransaction
+	TotalCount   int
+}
+
+// ListTransactions retrieves a paginated list of transactions with optional filters
+func (r *TransactionRepository) ListTransactions(filter *TransactionListFilter) (*TransactionListResult, error) {
+	// Build WHERE clause dynamically
+	conditions := []string{"1=1"}
+	args := []interface{}{}
+	argIndex := 1
+
+	if filter.CampaignSlug != "" {
+		conditions = append(conditions, fmt.Sprintf("campaign_slug = $%d", argIndex))
+		args = append(args, filter.CampaignSlug)
+		argIndex++
+	}
+	if filter.Status != "" {
+		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
+		args = append(args, filter.Status)
+		argIndex++
+	}
+	if filter.Provider != "" {
+		conditions = append(conditions, fmt.Sprintf("ad_provider = $%d", argIndex))
+		args = append(args, filter.Provider)
+		argIndex++
+	}
+	if filter.StartDate != nil {
+		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
+		args = append(args, *filter.StartDate)
+		argIndex++
+	}
+	if filter.EndDate != nil {
+		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
+		args = append(args, *filter.EndDate)
+		argIndex++
+	}
+
+	whereClause := ""
+	for i, cond := range conditions {
+		if i == 0 {
+			whereClause = "WHERE " + cond
+		} else {
+			whereClause += " AND " + cond
+		}
+	}
+
+	// Count query
+	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM acquisition_transactions %s`, whereClause)
+	var totalCount int
+	err := r.db.QueryRow(countQuery, args...).Scan(&totalCount)
+	if err != nil {
+		return nil, fmt.Errorf("failed to count transactions: %w", err)
+	}
+
+	// Apply defaults for pagination
+	limit := filter.Limit
+	if limit <= 0 || limit > 100 {
+		limit = 20
+	}
+	offset := filter.Offset
+	if offset < 0 {
+		offset = 0
+	}
+
+	// Data query with pagination
+	dataQuery := fmt.Sprintf(`
+		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
+		       next_action_payload, ad_provider, click_id, attribution_data,
+		       ip_address, user_agent, consent_required, consent_checked,
+		       consent_version, consent_timestamp, landing_version_hash,
+		       timwe_transaction_id, transaction_auth_code, timwe_status,
+		       he_source, he_msisdn, he_operator,
+		       charged_at, charge_payout, conversion_postback_sent,
+		       created_at, updated_at
+		FROM acquisition_transactions
+		%s
+		ORDER BY created_at DESC
+		LIMIT $%d OFFSET $%d
+	`, whereClause, argIndex, argIndex+1)
+
+	args = append(args, limit, offset)
+
+	rows, err := r.db.Query(dataQuery, args...)
+	if err != nil {
+		return nil, fmt.Errorf("failed to list transactions: %w", err)
+	}
+	defer rows.Close()
+
+	var transactions []*domain.AcquisitionTransaction
+	for rows.Next() {
+		tx, err := r.scanTransactionFromRow(rows)
+		if err != nil {
+			r.logger.Error("Failed to scan transaction row", zap.Error(err))
+			continue
+		}
+		transactions = append(transactions, tx)
+	}
+
+	return &TransactionListResult{
+		Transactions: transactions,
+		TotalCount:   totalCount,
+	}, nil
+}
+
+// scanTransactionFromRow scans a single transaction from sql.Rows
+func (r *TransactionRepository) scanTransactionFromRow(rows *sql.Rows) (*domain.AcquisitionTransaction, error) {
+	var tx domain.AcquisitionTransaction
+	var nextAction, adProvider, clickID, ipAddress, userAgent, consentVersion,
+		landingVersionHash, timweTransactionID,
+		transactionAuthCode, timweStatus, heSource, heMSISDN, heOperator, chargePayout sql.NullString
+	var consentTimestamp, chargedAt sql.NullTime
+	var nextActionPayload, attributionData sql.NullString
+
+	err := rows.Scan(
+		&tx.ID, &tx.CorrelationID, &tx.CampaignSlug, &tx.MSISDN, &tx.Status,
+		&nextAction, &nextActionPayload, &adProvider, &clickID, &attributionData,
+		&ipAddress, &userAgent, &tx.ConsentRequired, &tx.ConsentChecked,
+		&consentVersion, &consentTimestamp, &landingVersionHash,
+		&timweTransactionID, &transactionAuthCode, &timweStatus,
+		&heSource, &heMSISDN, &heOperator,
+		&chargedAt, &chargePayout, &tx.ConversionPostbackSent,
+		&tx.CreatedAt, &tx.UpdatedAt,
+	)
+
+	if err != nil {
+		return nil, fmt.Errorf("failed to scan transaction: %w", err)
+	}
+
+	// Map nullable fields
+	if nextAction.Valid {
+		na := domain.NextAction(nextAction.String)
+		tx.NextAction = &na
+	}
+	if adProvider.Valid {
+		tx.AdProvider = &adProvider.String
+	}
+	if clickID.Valid {
+		tx.ClickID = &clickID.String
+	}
+	if ipAddress.Valid {
+		tx.IPAddress = &ipAddress.String
+	}
+	if userAgent.Valid {
+		tx.UserAgent = &userAgent.String
+	}
+	if consentVersion.Valid {
+		tx.ConsentVersion = &consentVersion.String
+	}
+	if consentTimestamp.Valid {
+		tx.ConsentTimestamp = &consentTimestamp.Time
+	}
+	if landingVersionHash.Valid {
+		tx.LandingVersionHash = &landingVersionHash.String
+	}
+	if timweTransactionID.Valid {
+		tx.TimweTransactionID = &timweTransactionID.String
+	}
+	if transactionAuthCode.Valid {
+		tx.TransactionAuthCode = &transactionAuthCode.String
+	}
+	if timweStatus.Valid {
+		tx.TimweStatus = &timweStatus.String
+	}
+	if heSource.Valid {
+		src := domain.HESource(heSource.String)
+		tx.HESource = &src
+	}
+	if heMSISDN.Valid {
+		tx.HEMSISDN = &heMSISDN.String
+	}
+	if heOperator.Valid {
+		tx.HEOperator = &heOperator.String
+	}
+	if chargedAt.Valid {
+		tx.ChargedAt = &chargedAt.Time
+	}
+	if chargePayout.Valid {
+		tx.ChargePayout = &chargePayout.String
+	}
+	if nextActionPayload.Valid {
+		tx.NextActionPayload = json.RawMessage(nextActionPayload.String)
+	}
+	if attributionData.Valid {
+		tx.AttributionData = json.RawMessage(attributionData.String)
+	}
+
+	return &tx, nil
+}
diff --git a/services/acquisition-api/internal/service/transaction_service.go b/services/acquisition-api/internal/service/transaction_service.go
new file mode 100644
index 0000000..072eee4
--- /dev/null
+++ b/services/acquisition-api/internal/service/transaction_service.go
@@ -0,0 +1,551 @@
+package service
+
+import (
+	"encoding/json"
+	"fmt"
+	"net/http"
+	"time"
+
+	"github.com/google/uuid"
+	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
+	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
+	"go.uber.org/zap"
+)
+
+// TransactionService handles acquisition transaction business logic
+type TransactionService struct {
+	txRepo           *repository.TransactionRepository
+	campaignRepo     *repository.CampaignRepository
+	postbackRepo     *repository.PostbackRepository
+	providerReg      *ProviderRegistry
+	postbackTemplate *PostbackTemplateService
+	timweClient      TIMWEClient // Will be implemented to call TIMWE API
+	logger           *zap.Logger
+}
+
+// TIMWEClient interface for TIMWE API integration
+type TIMWEClient interface {
+	OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error)
+	Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error)
+}
+
+// TIMWEResponse represents a response from TIMWE API
+type TIMWEResponse struct {
+	Success             bool
+	TransactionID       string
+	TransactionAuthCode string
+	Status              string
+	RequiresConfirm     bool
+	Message             string
+}
+
+// NewTransactionService creates a new transaction service
+func NewTransactionService(
+	txRepo *repository.TransactionRepository,
+	campaignRepo *repository.CampaignRepository,
+	postbackRepo *repository.PostbackRepository,
+	providerReg *ProviderRegistry,
+	timweClient TIMWEClient,
+	logger *zap.Logger,
+) *TransactionService {
+	return &TransactionService{
+		txRepo:           txRepo,
+		campaignRepo:     campaignRepo,
+		postbackRepo:     postbackRepo,
+		providerReg:      providerReg,
+		postbackTemplate: NewPostbackTemplateService(logger),
+		timweClient:      timweClient,
+		logger:           logger,
+	}
+}
+
+// CreateTransaction creates a new acquisition transaction
+func (s *TransactionService) CreateTransaction(req *domain.CreateTransactionRequest) (*domain.CreateTransactionResponse, error) {
+	// Get campaign
+	campaign, err := s.campaignRepo.GetBySlug(req.CampaignSlug)
+	if err != nil {
+		return nil, fmt.Errorf("campaign not found: %w", err)
+	}
+
+	// Normalize attribution
+	var attribution *domain.Attribution
+	if req.Provider != nil && *req.Provider != "" {
+		provider, err := s.providerReg.Get(*req.Provider)
+		if err != nil {
+			s.logger.Warn("Provider not found, using generic", zap.String("provider", *req.Provider), zap.Error(err))
+			provider, _ = s.providerReg.Get("generic")
+		}
+
+		// Convert attribution data to map[string]string
+		attrMap := make(map[string]string)
+		for k, v := range req.AttributionData {
+			attrMap[k] = v
+		}
+		if req.ClickID != nil {
+			attrMap["click_id"] = *req.ClickID
+		}
+
+		attribution, err = provider.Normalize(attrMap)
+		if err != nil {
+			return nil, fmt.Errorf("failed to normalize attribution: %w", err)
+		}
+		attribution.CampaignSlug = req.CampaignSlug
+	} else {
+		// Generic attribution
+		attribution = &domain.Attribution{
+			Provider:     "generic",
+			CampaignSlug: req.CampaignSlug,
+		}
+		if req.ClickID != nil {
+			attribution.ClickID = *req.ClickID
+		}
+	}
+
+	// Check for duplicate (idempotency)
+	if attribution.ClickID != "" && attribution.Provider != "" {
+		existing, err := s.txRepo.FindByClickID(attribution.Provider, attribution.ClickID)
+		if err == nil && existing != nil {
+			// Return existing transaction
+			return s.buildResponse(existing), nil
+		}
+	}
+
+	// Check throttles
+	throttles := make(map[string]interface{})
+	if len(campaign.Throttles) > 0 {
+		json.Unmarshal(campaign.Throttles, &throttles)
+	}
+
+	ipAddr := ""
+	if req.IPAddress != nil {
+		ipAddr = *req.IPAddress
+	}
+
+	throttled, err := s.txRepo.CheckThrottle(campaign.Slug, req.MSISDN, ipAddr, throttles)
+	if err != nil {
+		return nil, fmt.Errorf("failed to check throttle: %w", err)
+	}
+	if throttled {
+		return nil, fmt.Errorf("request throttled")
+	}
+
+	// Validate consent if required
+	if campaign.ConsentRequired && !req.ConsentChecked {
+		return nil, fmt.Errorf("consent required but not checked")
+	}
+
+	// Create transaction
+	correlationID := uuid.New()
+	transactionID := uuid.New()
+
+	// Determine which MSISDN to use: HE-detected or form-submitted
+	msisdnToUse := req.MSISDN
+	if req.HESource != nil && *req.HESource != domain.HESourceNone && req.HEMSISDN != nil && *req.HEMSISDN != "" {
+		// Use HE-detected MSISDN (trusted from MNO or simulation)
+		msisdnToUse = *req.HEMSISDN
+		s.logger.Info("Using HE identity for transaction",
+			zap.String("he_source", string(*req.HESource)),
+			zap.String("form_msisdn_prefix", req.MSISDN[:min(5, len(req.MSISDN))]),
+			zap.String("he_msisdn_prefix", msisdnToUse[:min(5, len(msisdnToUse))]),
+		)
+	}
+
+	tx := &domain.AcquisitionTransaction{
+		ID:              transactionID,
+		CorrelationID:   correlationID,
+		CampaignSlug:    req.CampaignSlug,
+		MSISDN:          msisdnToUse, // Use HE MSISDN if available
+		Status:          domain.StatusPending,
+		AdProvider:      &attribution.Provider,
+		ClickID:         &attribution.ClickID,
+		IPAddress:       req.IPAddress,
+		UserAgent:       req.UserAgent,
+		ConsentRequired: campaign.ConsentRequired,
+		ConsentChecked:  req.ConsentChecked,
+		CreatedAt:       time.Now(),
+		UpdatedAt:       time.Now(),
+		// HE tracking fields
+		HESource:   req.HESource,
+		HEMSISDN:   req.HEMSISDN,
+		HEOperator: req.HEOperator,
+	}
+
+	if campaign.ConsentVersion != nil {
+		tx.ConsentVersion = campaign.ConsentVersion
+		if req.ConsentChecked {
+			now := time.Now()
+			tx.ConsentTimestamp = &now
+		}
+	}
+
+	// Store attribution data
+	attrData, _ := json.Marshal(attribution)
+	tx.AttributionData = attrData
+
+	// Call TIMWE API
+	partnerRoleID := ""
+	if campaign.PartnerRoleID != nil && *campaign.PartnerRoleID > 0 {
+		partnerRoleID = fmt.Sprintf("%d", *campaign.PartnerRoleID)
+	}
+	timweResp, err := s.timweClient.OptIn(
+		msisdnToUse, // Use HE MSISDN if available, otherwise form MSISDN
+		campaign.OfferProductID,
+		"WEB",
+		map[string]string{
+			"click_id": attribution.ClickID,
+			"campaign": attribution.CampaignSlug,
+		},
+		partnerRoleID,
+	)
+
+	if err != nil {
+		tx.Status = domain.StatusFailed
+		s.txRepo.Create(tx)
+		return nil, fmt.Errorf("TIMWE opt-in failed: %w", err)
+	}
+
+	// Update transaction with TIMWE response
+	if timweResp.TransactionID != "" {
+		tx.TimweTransactionID = &timweResp.TransactionID
+	}
+	if timweResp.TransactionAuthCode != "" {
+		tx.TransactionAuthCode = &timweResp.TransactionAuthCode
+	}
+	if timweResp.Status != "" {
+		tx.TimweStatus = &timweResp.Status
+	}
+
+	// Determine next action based on campaign flow type and TIMWE response
+	var nextAction domain.NextAction
+	var payload map[string]interface{}
+
+	if timweResp.RequiresConfirm {
+		tx.Status = domain.StatusConfirmRequired
+		nextAction = domain.NextActionOTP
+		payload = map[string]interface{}{
+			"transaction_id": tx.ID.String(),
+			"prompt":         "Please enter the confirmation code sent to your phone",
+		}
+	} else if campaign.FlowType == domain.FlowTypeClickToSMS && campaign.ShortCode != nil && campaign.SMSKeyword != nil {
+		tx.Status = domain.StatusActionRequired
+		nextAction = domain.NextActionOpenSMS
+		smsLink := fmt.Sprintf("sms:%s?body=%s", *campaign.ShortCode, *campaign.SMSKeyword)
+		payload = map[string]interface{}{
+			"sms_link":   smsLink,
+			"short_code": *campaign.ShortCode,
+			"keyword":    *campaign.SMSKeyword,
+			"fallback_steps": []string{
+				"Open your SMS app",
+				fmt.Sprintf("Send '%s' to %s", *campaign.SMSKeyword, *campaign.ShortCode),
+				"Wait for confirmation",
+			},
+		}
+	} else if timweResp.Success {
+		tx.Status = domain.StatusSubscribed
+		nextAction = domain.NextActionShowInstructions
+		payload = map[string]interface{}{
+			"message": "Subscription successful!",
+		}
+		// NOTE: Conversion postback is NOT fired here. It fires on charge success
+		// via HandleChargeSuccess() when subscription-external notifies us.
+		// This is the Mobplus requirement: postback only on actual charge.
+	} else {
+		tx.Status = domain.StatusFailed
+		nextAction = domain.NextActionShowInstructions
+		payload = map[string]interface{}{
+			"message": "Subscription failed. Please try again.",
+		}
+	}
+
+	tx.NextAction = &nextAction
+	payloadJSON, _ := json.Marshal(payload)
+	tx.NextActionPayload = payloadJSON
+
+	// Save transaction
+	err = s.txRepo.Create(tx)
+	if err != nil {
+		return nil, fmt.Errorf("failed to save transaction: %w", err)
+	}
+
+	return s.buildResponse(tx), nil
+}
+
+// ConfirmTransaction confirms a transaction (OTP flow)
+func (s *TransactionService) ConfirmTransaction(transactionID uuid.UUID, authCode string) (*domain.TransactionStatusResponse, error) {
+	// Get transaction
+	tx, err := s.txRepo.GetByID(transactionID)
+	if err != nil {
+		return nil, fmt.Errorf("transaction not found: %w", err)
+	}
+
+	if tx.Status != domain.StatusConfirmRequired {
+		return nil, fmt.Errorf("transaction is not in confirm_required status")
+	}
+
+	// Call TIMWE confirm
+	if tx.TimweTransactionID == nil {
+		return nil, fmt.Errorf("missing TIMWE transaction data")
+	}
+
+	// Fetch campaign to get product + partner role (confirm endpoint requires these)
+	campaign, err := s.campaignRepo.GetBySlug(tx.CampaignSlug)
+	if err != nil {
+		return nil, fmt.Errorf("campaign not found: %w", err)
+	}
+	partnerRoleID := ""
+	if campaign.PartnerRoleID != nil && *campaign.PartnerRoleID > 0 {
+		partnerRoleID = fmt.Sprintf("%d", *campaign.PartnerRoleID)
+	}
+
+	timweResp, err := s.timweClient.Confirm(tx.MSISDN, campaign.OfferProductID, "WEB", partnerRoleID, authCode)
+	if err != nil {
+		return nil, fmt.Errorf("TIMWE confirm failed: %w", err)
+	}
+
+	if !timweResp.Success {
+		// Update status to failed
+		s.txRepo.UpdateStatus(transactionID, domain.StatusFailed, nil, nil)
+		return &domain.TransactionStatusResponse{
+			TransactionID: transactionID,
+			Status:        domain.StatusFailed,
+		}, nil
+	}
+
+	// Update to subscribed
+	s.txRepo.UpdateStatus(transactionID, domain.StatusSubscribed, nil, nil)
+	if timweResp.Status != "" {
+		s.txRepo.UpdateTIMWEData(transactionID, *tx.TimweTransactionID, authCode, timweResp.Status)
+	}
+
+	// NOTE: Conversion postback is NOT fired here. It fires on charge success
+	// via HandleChargeSuccess() when subscription-external notifies us.
+
+	return &domain.TransactionStatusResponse{
+		TransactionID: transactionID,
+		Status:        domain.StatusSubscribed,
+		NextAction:    nil,
+		Payload:       map[string]interface{}{"message": "Subscription confirmed successfully"},
+	}, nil
+}
+
+// GetTransactionStatus retrieves the current status of a transaction
+func (s *TransactionService) GetTransactionStatus(transactionID uuid.UUID) (*domain.TransactionStatusResponse, error) {
+	tx, err := s.txRepo.GetByID(transactionID)
+	if err != nil {
+		return nil, fmt.Errorf("transaction not found: %w", err)
+	}
+
+	var payload map[string]interface{}
+	if len(tx.NextActionPayload) > 0 {
+		json.Unmarshal(tx.NextActionPayload, &payload)
+	}
+
+	return &domain.TransactionStatusResponse{
+		TransactionID: tx.ID,
+		Status:        tx.Status,
+		NextAction:    tx.NextAction,
+		Payload:       payload,
+	}, nil
+}
+
+// buildResponse builds a CreateTransactionResponse from a transaction
+func (s *TransactionService) buildResponse(tx *domain.AcquisitionTransaction) *domain.CreateTransactionResponse {
+	var payload map[string]interface{}
+	if len(tx.NextActionPayload) > 0 {
+		json.Unmarshal(tx.NextActionPayload, &payload)
+	}
+
+	var nextAction domain.NextAction
+	if tx.NextAction != nil {
+		nextAction = *tx.NextAction
+	}
+
+	return &domain.CreateTransactionResponse{
+		TransactionID: tx.ID,
+		CorrelationID: tx.CorrelationID,
+		Status:        tx.Status,
+		NextAction:    nextAction,
+		Payload:       payload,
+	}
+}
+
+// enqueuePostback enqueues a postback for async delivery using campaign templates
+func (s *TransactionService) enqueuePostback(tx *domain.AcquisitionTransaction, event domain.PostbackEvent, attribution *domain.Attribution, campaign *domain.Campaign) {
+	if attribution == nil || attribution.Provider == "" {
+		s.logger.Debug("Skipping postback: no provider")
+		return
+	}
+
+	// Build postback context
+	ctx := domain.NewPostbackContext(tx, attribution)
+
+	// Add payout if available
+	if tx.ChargePayout != nil {
+		ctx.Payout = *tx.ChargePayout
+	}
+
+	var req *http.Request
+	var err error
+
+	// Try template-driven postback first (preferred)
+	if campaign != nil && len(campaign.PostbackRules) > 0 {
+		rules, parseErr := s.postbackTemplate.ParsePostbackRules(campaign.PostbackRules)
+		if parseErr == nil && rules != nil {
+			template, found := s.postbackTemplate.GetTemplateForEvent(rules, event, attribution.Provider)
+			if found {
+				req, err = s.postbackTemplate.BuildPostbackFromTemplate(template, ctx)
+				if err != nil {
+					s.logger.Error("Failed to build postback from template",
+						zap.String("event", string(event)),
+						zap.String("provider", attribution.Provider),
+						zap.Error(err))
+					return
+				}
+			}
+		}
+	}
+
+	// Fallback to legacy provider-based postback if no template found
+	if req == nil {
+		provider, providerErr := s.providerReg.Get(attribution.Provider)
+		if providerErr != nil {
+			s.logger.Warn("Provider not found for postback", zap.String("provider", attribution.Provider))
+			return
+		}
+
+		outcome := map[string]interface{}{
+			"transaction_id": tx.ID.String(),
+			"status":         string(tx.Status),
+			"msisdn":         tx.MSISDN,
+		}
+
+		req, err = provider.BuildPostback(event, attribution, outcome)
+		if err != nil {
+			s.logger.Error("Failed to build postback", zap.Error(err))
+			return
+		}
+	}
+
+	// Create outbox entry
+	outbox := &domain.PostbackOutbox{
+		ID:                  uuid.New(),
+		TransactionID:       tx.ID,
+		Event:               event,
+		Provider:            attribution.Provider,
+		URLTemplateRendered: req.URL.String(),
+		HTTPMethod:          req.Method,
+		AttemptCount:        0,
+		MaxAttempts:         5,
+		Status:              domain.PostbackStatusPending,
+		CreatedAt:           time.Now(),
+		UpdatedAt:           time.Now(),
+	}
+
+	// Serialize headers
+	headersJSON, _ := json.Marshal(req.Header)
+	outbox.Headers = string(headersJSON)
+
+	err = s.postbackRepo.CreateOutbox(outbox)
+	if err != nil {
+		s.logger.Error("Failed to enqueue postback", zap.Error(err))
+	} else {
+		s.logger.Info("Postback enqueued",
+			zap.String("transaction_id", tx.ID.String()),
+			zap.String("event", string(event)),
+			zap.String("provider", attribution.Provider),
+			zap.String("url", req.URL.String()),
+		)
+	}
+}
+
+// ChargeSuccessRequest represents the request from subscription-external on charge success
+type ChargeSuccessRequest struct {
+	TimweTransactionID string `json:"timwe_transaction_id"`
+	MSISDN             string `json:"msisdn,omitempty"`
+	ProductID          int    `json:"product_id,omitempty"`
+	ChargedAt          string `json:"charged_at,omitempty"`
+	Payout             string `json:"payout,omitempty"`
+}
+
+// HandleChargeSuccess processes a charge success notification and enqueues conversion postback
+func (s *TransactionService) HandleChargeSuccess(req *ChargeSuccessRequest) error {
+	if req.TimweTransactionID == "" {
+		return fmt.Errorf("timwe_transaction_id is required")
+	}
+
+	// Find transaction by TIMWE transaction ID
+	tx, err := s.txRepo.FindByTimweTransactionID(req.TimweTransactionID)
+	if err != nil {
+		// Fallback: try by MSISDN if provided
+		if req.MSISDN != "" {
+			tx, err = s.txRepo.FindByMSISDNAndStatus(req.MSISDN, domain.StatusSubscribed)
+			if err != nil {
+				return fmt.Errorf("transaction not found for timwe_transaction_id=%s: %w", req.TimweTransactionID, err)
+			}
+		} else {
+			return fmt.Errorf("transaction not found for timwe_transaction_id=%s: %w", req.TimweTransactionID, err)
+		}
+	}
+
+	// Check if already processed (idempotency)
+	if tx.ConversionPostbackSent {
+		s.logger.Info("Conversion postback already sent, skipping",
+			zap.String("transaction_id", tx.ID.String()),
+			zap.String("timwe_transaction_id", req.TimweTransactionID),
+		)
+		return nil
+	}
+
+	// Update transaction to CHARGED status
+	now := time.Now()
+	tx.Status = domain.StatusCharged
+	tx.ChargedAt = &now
+	if req.Payout != "" {
+		tx.ChargePayout = &req.Payout
+	}
+
+	// Mark postback as pending to be sent
+	if err := s.txRepo.MarkCharged(tx.ID, &now, req.Payout); err != nil {
+		return fmt.Errorf("failed to mark transaction as charged: %w", err)
+	}
+
+	// Get campaign for postback rules
+	campaign, err := s.campaignRepo.GetBySlug(tx.CampaignSlug)
+	if err != nil {
+		s.logger.Warn("Campaign not found for postback rules",
+			zap.String("campaign_slug", tx.CampaignSlug),
+			zap.Error(err))
+		// Continue anyway, will use fallback postback logic
+	}
+
+	// Parse attribution data
+	var attribution domain.Attribution
+	if len(tx.AttributionData) > 0 {
+		if err := json.Unmarshal(tx.AttributionData, &attribution); err != nil {
+			s.logger.Warn("Failed to parse attribution data", zap.Error(err))
+		}
+	}
+
+	// Enqueue conversion postback (Mobplus requirement: fire on charge success)
+	s.enqueuePostback(tx, domain.PostbackEventConversion, &attribution, campaign)
+
+	// Mark conversion postback as sent
+	if err := s.txRepo.MarkConversionPostbackSent(tx.ID); err != nil {
+		s.logger.Error("Failed to mark conversion postback sent", zap.Error(err))
+		// Don't return error - postback is already enqueued
+	}
+
+	s.logger.Info("Charge success processed, conversion postback enqueued",
+		zap.String("transaction_id", tx.ID.String()),
+		zap.String("timwe_transaction_id", req.TimweTransactionID),
+		zap.String("provider", attribution.Provider),
+		zap.String("click_id", attribution.ClickID),
+	)
+
+	return nil
+}
+
+// GetTransactionByTimweID retrieves a transaction by TIMWE transaction ID
+func (s *TransactionService) GetTransactionByTimweID(timweTransactionID string) (*domain.AcquisitionTransaction, error) {
+	return s.txRepo.FindByTimweTransactionID(timweTransactionID)
+}
diff --git a/services/billing/go.mod b/services/billing/go.mod
new file mode 100644
index 0000000..29e6f12
--- /dev/null
+++ b/services/billing/go.mod
@@ -0,0 +1,48 @@
+module github.com/seidu626/subscription-manager/billing
+
+go 1.24.2
+
+require (
+	github.com/dgrijalva/jwt-go v3.2.0+incompatible
+	github.com/fsnotify/fsnotify v1.7.0
+	github.com/lib/pq v1.10.9
+	github.com/prometheus/client_golang v1.20.4
+	github.com/prometheus/client_model v0.6.1
+	github.com/sony/gobreaker v1.0.0
+	github.com/spf13/viper v1.19.0
+	github.com/stretchr/testify v1.9.0
+	github.com/valyala/fasthttp v1.56.0
+	go.uber.org/zap v1.27.0
+)
+
+require (
+	github.com/andybalholm/brotli v1.1.0 // indirect
+	github.com/beorn7/perks v1.0.1 // indirect
+	github.com/cespare/xxhash/v2 v2.3.0 // indirect
+	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
+	github.com/hashicorp/hcl v1.0.0 // indirect
+	github.com/klauspost/compress v1.17.9 // indirect
+	github.com/magiconair/properties v1.8.7 // indirect
+	github.com/mitchellh/mapstructure v1.5.0 // indirect
+	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
+	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
+	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
+	github.com/prometheus/common v0.55.0 // indirect
+	github.com/prometheus/procfs v0.15.1 // indirect
+	github.com/sagikazarmark/locafero v0.4.0 // indirect
+	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
+	github.com/sourcegraph/conc v0.3.0 // indirect
+	github.com/spf13/afero v1.11.0 // indirect
+	github.com/spf13/cast v1.6.0 // indirect
+	github.com/spf13/pflag v1.0.5 // indirect
+	github.com/stretchr/objx v0.5.2 // indirect
+	github.com/subosito/gotenv v1.6.0 // indirect
+	github.com/valyala/bytebufferpool v1.0.0 // indirect
+	go.uber.org/multierr v1.10.0 // indirect
+	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
+	golang.org/x/sys v0.25.0 // indirect
+	golang.org/x/text v0.18.0 // indirect
+	google.golang.org/protobuf v1.34.2 // indirect
+	gopkg.in/ini.v1 v1.67.0 // indirect
+	gopkg.in/yaml.v3 v3.0.1 // indirect
+)
diff --git a/services/billing/go.sum b/services/billing/go.sum
new file mode 100644
index 0000000..06c2864
--- /dev/null
+++ b/services/billing/go.sum
@@ -0,0 +1,104 @@
+github.com/andybalholm/brotli v1.1.0 h1:eLKJA0d02Lf0mVpIDgYnqXcUn0GqVmEFny3VuID1U3M=
+github.com/andybalholm/brotli v1.1.0/go.mod h1:sms7XGricyQI9K10gOSf56VKKWS4oLer58Q+mhRPtnY=
+github.com/beorn7/perks v1.0.1 h1:VlbKKnNfV8bJzeqoa4cOKqO6bYr3WgKZxO8Z16+hsOM=
+github.com/beorn7/perks v1.0.1/go.mod h1:G2ZrVWU2WbWT9wwq4/hrbKbnv/1ERSJQ0ibhJ6rlkpw=
+github.com/cespare/xxhash/v2 v2.3.0 h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=
+github.com/cespare/xxhash/v2 v2.3.0/go.mod h1:VGX0DQ3Q6kWi7AoAeZDth3/j3BFtOZR5XLFGgcrjCOs=
+github.com/davecgh/go-spew v1.1.0/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
+github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
+github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc h1:U9qPSI2PIWSS1VwoXQT9A3Wy9MM3WgvqSxFWenqJduM=
+github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
+github.com/dgrijalva/jwt-go v3.2.0+incompatible h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=
+github.com/dgrijalva/jwt-go v3.2.0+incompatible/go.mod h1:E3ru+11k8xSBh+hMPgOLZmtrrCbhqsmaPHjLKYnJCaQ=
+github.com/frankban/quicktest v1.14.6 h1:7Xjx+VpznH+oBnejlPUj8oUpdxnVs4f8XU8WnHkI4W8=
+github.com/frankban/quicktest v1.14.6/go.mod h1:4ptaffx2x8+WTWXmUCuVU6aPUX1/Mz7zb5vbUoiM6w0=
+github.com/fsnotify/fsnotify v1.7.0 h1:8JEhPFa5W2WU7YfeZzPNqzMP6Lwt7L2715Ggo0nosvA=
+github.com/fsnotify/fsnotify v1.7.0/go.mod h1:40Bi/Hjc2AVfZrqy+aj+yEI+/bRxZnMJyTJwOpGvigM=
+github.com/google/go-cmp v0.6.0 h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI=
+github.com/google/go-cmp v0.6.0/go.mod h1:17dUlkBOakJ0+DkrSSNjCkIjxS6bF9zb3elmeNGIjoY=
+github.com/hashicorp/hcl v1.0.0 h1:0Anlzjpi4vEasTeNFn2mLJgTSwt0+6sfsiTG8qcWGx4=
+github.com/hashicorp/hcl v1.0.0/go.mod h1:E5yfLk+7swimpb2L/Alb/PJmXilQ/rhwaUYs4T20WEQ=
+github.com/klauspost/compress v1.17.9 h1:6KIumPrER1LHsvBVuDa0r5xaG0Es51mhhB9BQB2qeMA=
+github.com/klauspost/compress v1.17.9/go.mod h1:Di0epgTjJY877eYKx5yC51cX2A2Vl2ibi7bDH9ttBbw=
+github.com/kr/pretty v0.3.1 h1:flRD4NNwYAUpkphVc1HcthR4KEIFJ65n8Mw5qdRn3LE=
+github.com/kr/pretty v0.3.1/go.mod h1:hoEshYVHaxMs3cyo3Yncou5ZscifuDolrwPKZanG3xk=
+github.com/kr/text v0.2.0 h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=
+github.com/kr/text v0.2.0/go.mod h1:eLer722TekiGuMkidMxC/pM04lWEeraHUUmBw8l2grE=
+github.com/lib/pq v1.10.9 h1:YXG7RB+JIjhP29X+OtkiDnYaXQwpS4JEWq7dtCCRUEw=
+github.com/lib/pq v1.10.9/go.mod h1:AlVN5x4E4T544tWzH6hKfbfQvm3HdbOxrmggDNAPY9o=
+github.com/magiconair/properties v1.8.7 h1:IeQXZAiQcpL9mgcAe1Nu6cX9LLw6ExEHKjN0VQdvPDY=
+github.com/magiconair/properties v1.8.7/go.mod h1:Dhd985XPs7jluiymwWYZ0G4Z61jb3vdS329zhj2hYo0=
+github.com/mitchellh/mapstructure v1.5.0 h1:jeMsZIYE/09sWLaz43PL7Gy6RuMjD2eJVyuac5Z2hdY=
+github.com/mitchellh/mapstructure v1.5.0/go.mod h1:bFUtVrKA4DC2yAKiSyO/QUcy7e+RRV2QTWOzhPopBRo=
+github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 h1:C3w9PqII01/Oq1c1nUAm88MOHcQC9l5mIlSMApZMrHA=
+github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822/go.mod h1:+n7T8mK8HuQTcFwEeznm/DIxMOiR9yIdICNftLE1DvQ=
+github.com/pelletier/go-toml/v2 v2.2.2 h1:aYUidT7k73Pcl9nb2gScu7NSrKCSHIDE89b3+6Wq+LM=
+github.com/pelletier/go-toml/v2 v2.2.2/go.mod h1:1t835xjRzz80PqgE6HHgN2JOsmgYu/h4qDAS4n929Rs=
+github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
+github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 h1:Jamvg5psRIccs7FGNTlIRMkT8wgtp5eCXdBlqhYGL6U=
+github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
+github.com/prometheus/client_golang v1.20.4 h1:Tgh3Yr67PaOv/uTqloMsCEdeuFTatm5zIq5+qNN23vI=
+github.com/prometheus/client_golang v1.20.4/go.mod h1:PIEt8X02hGcP8JWbeHyeZ53Y/jReSnHgO035n//V5WE=
+github.com/prometheus/client_model v0.6.1 h1:ZKSh/rekM+n3CeS952MLRAdFwIKqeY8b62p8ais2e9E=
+github.com/prometheus/client_model v0.6.1/go.mod h1:OrxVMOVHjw3lKMa8+x6HeMGkHMQyHDk9E3jmP2AmGiY=
+github.com/prometheus/common v0.55.0 h1:KEi6DK7lXW/m7Ig5i47x0vRzuBsHuvJdi5ee6Y3G1dc=
+github.com/prometheus/common v0.55.0/go.mod h1:2SECS4xJG1kd8XF9IcM1gMX6510RAEL65zxzNImwdc8=
+github.com/prometheus/procfs v0.15.1 h1:YagwOFzUgYfKKHX6Dr+sHT7km/hxC76UB0learggepc=
+github.com/prometheus/procfs v0.15.1/go.mod h1:fB45yRUv8NstnjriLhBQLuOUt+WW4BsoGhij/e3PBqk=
+github.com/rogpeppe/go-internal v1.10.0 h1:TMyTOH3F/DB16zRVcYyreMH6GnZZrwQVAoYjRBZyWFQ=
+github.com/rogpeppe/go-internal v1.10.0/go.mod h1:UQnix2H7Ngw/k4C5ijL5+65zddjncjaFoBhdsK/akog=
+github.com/sagikazarmark/locafero v0.4.0 h1:HApY1R9zGo4DBgr7dqsTH/JJxLTTsOt7u6keLGt6kNQ=
+github.com/sagikazarmark/locafero v0.4.0/go.mod h1:Pe1W6UlPYUk/+wc/6KFhbORCfqzgYEpgQ3O5fPuL3H4=
+github.com/sagikazarmark/slog-shim v0.1.0 h1:diDBnUNK9N/354PgrxMywXnAwEr1QZcOr6gto+ugjYE=
+github.com/sagikazarmark/slog-shim v0.1.0/go.mod h1:SrcSrq8aKtyuqEI1uvTDTK1arOWRIczQRv+GVI1AkeQ=
+github.com/sony/gobreaker v1.0.0 h1:feX5fGGXSl3dYd4aHZItw+FpHLvvoaqkawKjVNiFMNQ=
+github.com/sony/gobreaker v1.0.0/go.mod h1:ZKptC7FHNvhBz7dN2LGjPVBz2sZJmc0/PkyDJOjmxWY=
+github.com/sourcegraph/conc v0.3.0 h1:OQTbbt6P72L20UqAkXXuLOj79LfEanQ+YQFNpLA9ySo=
+github.com/sourcegraph/conc v0.3.0/go.mod h1:Sdozi7LEKbFPqYX2/J+iBAM6HpqSLTASQIKqDmF7Mt0=
+github.com/spf13/afero v1.11.0 h1:WJQKhtpdm3v2IzqG8VMqrr6Rf3UYpEF239Jy9wNepM8=
+github.com/spf13/afero v1.11.0/go.mod h1:GH9Y3pIexgf1MTIWtNGyogA5MwRIDXGUr+hbWNoBjkY=
+github.com/spf13/cast v1.6.0 h1:GEiTHELF+vaR5dhz3VqZfFSzZjYbgeKDpBxQVS4GYJ0=
+github.com/spf13/cast v1.6.0/go.mod h1:ancEpBxwJDODSW/UG4rDrAqiKolqNNh2DX3mk86cAdo=
+github.com/spf13/pflag v1.0.5 h1:iy+VFUOCP1a+8yFto/drg2CJ5u0yRoB7fZw3DKv/JXA=
+github.com/spf13/pflag v1.0.5/go.mod h1:McXfInJRrz4CZXVZOBLb0bTZqETkiAhM9Iw0y3An2Bg=
+github.com/spf13/viper v1.19.0 h1:RWq5SEjt8o25SROyN3z2OrDB9l7RPd3lwTWU8EcEdcI=
+github.com/spf13/viper v1.19.0/go.mod h1:GQUN9bilAbhU/jgc1bKs99f/suXKeUMct8Adx5+Ntkg=
+github.com/stretchr/objx v0.1.0/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
+github.com/stretchr/objx v0.4.0/go.mod h1:YvHI0jy2hoMjB+UWwv71VJQ9isScKT/TqJzVSSt89Yw=
+github.com/stretchr/objx v0.5.0/go.mod h1:Yh+to48EsGEfYuaHDzXPcE3xhTkx73EhmCGUpEOglKo=
+github.com/stretchr/objx v0.5.2 h1:xuMeJ0Sdp5ZMRXx/aWO6RZxdr3beISkG5/G/aIRr3pY=
+github.com/stretchr/objx v0.5.2/go.mod h1:FRsXN1f5AsAjCGJKqEizvkpNtU+EGNCLh3NxZ/8L+MA=
+github.com/stretchr/testify v1.3.0/go.mod h1:M5WIy9Dh21IEIfnGCwXGc5bZfKNJtfHm1UVUgZn+9EI=
+github.com/stretchr/testify v1.7.1/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
+github.com/stretchr/testify v1.8.0/go.mod h1:yNjHg4UonilssWZ8iaSj1OCr/vHnekPRkoO+kdMU+MU=
+github.com/stretchr/testify v1.8.4/go.mod h1:sz/lmYIOXD/1dqDmKjjqLyZ2RngseejIcXlSw2iwfAo=
+github.com/stretchr/testify v1.9.0 h1:HtqpIVDClZ4nwg75+f6Lvsy/wHu+3BoSGCbBAcpTsTg=
+github.com/stretchr/testify v1.9.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
+github.com/subosito/gotenv v1.6.0 h1:9NlTDc1FTs4qu0DDq7AEtTPNw6SVm7uBMsUCUjABIf8=
+github.com/subosito/gotenv v1.6.0/go.mod h1:Dk4QP5c2W3ibzajGcXpNraDfq2IrhjMIvMSWPKKo0FU=
+github.com/valyala/bytebufferpool v1.0.0 h1:GqA5TC/0021Y/b9FG4Oi9Mr3q7XYx6KllzawFIhcdPw=
+github.com/valyala/bytebufferpool v1.0.0/go.mod h1:6bBcMArwyJ5K/AmCkWv1jt77kVWyCJ6HpOuEn7z0Csc=
+github.com/valyala/fasthttp v1.56.0 h1:bEZdJev/6LCBlpdORfrLu/WOZXXxvrUQSiyniuaoW8U=
+github.com/valyala/fasthttp v1.56.0/go.mod h1:sReBt3XZVnudxuLOx4J/fMrJVorWRiWY2koQKgABiVI=
+go.uber.org/goleak v1.3.0 h1:2K3zAYmnTNqV73imy9J1T3WC+gmCePx2hEGkimedGto=
+go.uber.org/goleak v1.3.0/go.mod h1:CoHD4mav9JJNrW/WLlf7HGZPjdw8EucARQHekz1X6bE=
+go.uber.org/multierr v1.10.0 h1:S0h4aNzvfcFsC3dRF1jLoaov7oRaKqRGC/pUEJ2yvPQ=
+go.uber.org/multierr v1.10.0/go.mod h1:20+QtiLqy0Nd6FdQB9TLXag12DsQkrbs3htMFfDN80Y=
+go.uber.org/zap v1.27.0 h1:aJMhYGrd5QSmlpLMr2MftRKl7t8J8PTZPA732ud/XR8=
+go.uber.org/zap v1.27.0/go.mod h1:GB2qFLM7cTU87MWRP2mPIjqfIDnGu+VIO4V/SdhGo2E=
+golang.org/x/exp v0.0.0-20230905200255-921286631fa9 h1:GoHiUyI/Tp2nVkLI2mCxVkOjsbSXD66ic0XW0js0R9g=
+golang.org/x/exp v0.0.0-20230905200255-921286631fa9/go.mod h1:S2oDrQGGwySpoQPVqRShND87VCbxmc6bL1Yd2oYrm6k=
+golang.org/x/sys v0.25.0 h1:r+8e+loiHxRqhXVl6ML1nO3l1+oFoWbnlu2Ehimmi34=
+golang.org/x/sys v0.25.0/go.mod h1:/VUhepiaJMQUp4+oa/7Zr1D23ma6VTLIYjOOTFZPUcA=
+golang.org/x/text v0.18.0 h1:XvMDiNzPAl0jr17s6W9lcaIhGUfUORdGCNsuLmPG224=
+golang.org/x/text v0.18.0/go.mod h1:BuEKDfySbSR4drPmRPG/7iBdf8hvFMuRexcpahXilzY=
+google.golang.org/protobuf v1.34.2 h1:6xV6lTsCfpGD21XK49h7MhtcApnLqkfYgPcdHftf6hg=
+google.golang.org/protobuf v1.34.2/go.mod h1:qYOHts0dSfpeUzUFpOMr/WGzszTmLH+DiWniOlNbLDw=
+gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
+gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c h1:Hei/4ADfdWqJk1ZMxUNpqntNwaWcugrBjAiHlqqRiVk=
+gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c/go.mod h1:JHkPIbrfpd72SG/EVd6muEfDQjcINNoR0C8j2r3qZ4Q=
+gopkg.in/ini.v1 v1.67.0 h1:Dgnx+6+nfE+IfzjUEISNeydPJh9AXNNsWbGP9KzCsOA=
+gopkg.in/ini.v1 v1.67.0/go.mod h1:pNLf8WUiyNEtQjuu5G5vTm06TEv9tsIgeAvK8hOrP4k=
+gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
+gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
+gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
diff --git a/services/billing/internal/handler/health.go b/services/billing/internal/handler/health.go
new file mode 100644
index 0000000..6aa9a35
--- /dev/null
+++ b/services/billing/internal/handler/health.go
@@ -0,0 +1,18 @@
+package handler
+
+import (
+	"github.com/valyala/fasthttp"
+)
+
+// HealthHandler handles health check requests for fasthttp
+func HealthHandler(ctx *fasthttp.RequestCtx) {
+	ctx.SetContentType("text/plain")
+	ctx.SetStatusCode(fasthttp.StatusOK)
+	ctx.SetBodyString("OK")
+}
+
+// HealthCheck handles health check requests for net/http (legacy)
+// Deprecated: Use HealthHandler for fasthttp
+func HealthCheck(ctx *fasthttp.RequestCtx) {
+	HealthHandler(ctx)
+}
diff --git a/services/billing/internal/handler/http.go b/services/billing/internal/handler/http.go
new file mode 100644
index 0000000..dd7a4ff
--- /dev/null
+++ b/services/billing/internal/handler/http.go
@@ -0,0 +1,92 @@
+package handler
+
+import (
+	"encoding/json"
+	"strconv"
+	"strings"
+
+	"github.com/seidu626/subscription-manager/billing/internal/service"
+	"github.com/valyala/fasthttp"
+)
+
+type BillingHandler struct {
+	service *service.BillingService
+}
+
+func NewBillingHandler(service *service.BillingService) *BillingHandler {
+	return &BillingHandler{service: service}
+}
+
+// ProcessPayment handles payment processing requests
+func (h *BillingHandler) ProcessPayment(ctx *fasthttp.RequestCtx) {
+	var req struct {
+		MSISDN    string  `json:"msisdn"`
+		ProductID int     `json:"product_id"`
+		Amount    float64 `json:"amount"`
+	}
+
+	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
+		ctx.SetStatusCode(fasthttp.StatusBadRequest)
+		ctx.SetBodyString("Invalid request payload")
+		return
+	}
+
+	tx, err := h.service.ProcessPayment(req.MSISDN, req.ProductID, req.Amount)
+	if err != nil {
+		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
+		ctx.SetBodyString("Failed to process payment")
+		return
+	}
+
+	ctx.SetContentType("application/json")
+	json.NewEncoder(ctx).Encode(tx)
+}
+
+// ListTransactions handles GET /api/v1/billing/transactions
+func (h *BillingHandler) ListTransactions(ctx *fasthttp.RequestCtx) {
+	msisdn := string(ctx.QueryArgs().Peek("msisdn"))
+	if msisdn == "" {
+		ctx.SetStatusCode(fasthttp.StatusBadRequest)
+		ctx.SetBodyString("msisdn query parameter is required")
+		return
+	}
+
+	transactions, err := h.service.FindByMSISDN(msisdn)
+	if err != nil {
+		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
+		ctx.SetBodyString("Failed to list transactions")
+		return
+	}
+
+	ctx.SetContentType("application/json")
+	json.NewEncoder(ctx).Encode(transactions)
+}
+
+// CreateTransaction handles POST /api/v1/billing/transactions
+func (h *BillingHandler) CreateTransaction(ctx *fasthttp.RequestCtx) {
+	h.ProcessPayment(ctx)
+}
+
+// GetTransaction handles GET /api/v1/billing/transaction/:id
+func (h *BillingHandler) GetTransaction(ctx *fasthttp.RequestCtx) {
+	path := string(ctx.Path())
+	// Extract ID from path /api/v1/billing/transaction/:id
+	parts := strings.Split(path, "/")
+	if len(parts) < 5 {
+		ctx.SetStatusCode(fasthttp.StatusBadRequest)
+		ctx.SetBodyString("Invalid transaction ID")
+		return
+	}
+
+	idStr := parts[len(parts)-1]
+	_, err := strconv.Atoi(idStr)
+	if err != nil {
+		ctx.SetStatusCode(fasthttp.StatusBadRequest)
+		ctx.SetBodyString("Invalid transaction ID format")
+		return
+	}
+
+	// For now, return not implemented as we don't have GetByID in repository
+	ctx.SetStatusCode(fasthttp.StatusNotImplemented)
+	ctx.SetBodyString("Get transaction by ID not implemented")
+}
diff --git a/services/billing/internal/handler/metrics.go b/services/billing/internal/handler/metrics.go
new file mode 100644
index 0000000..d2bc5fb
--- /dev/null
+++ b/services/billing/internal/handler/metrics.go
@@ -0,0 +1,94 @@
+package handler
+
+import (
+	"fmt"
+
+	"github.com/prometheus/client_golang/prometheus"
+	io_prometheus_client "github.com/prometheus/client_model/go"
+	"github.com/valyala/fasthttp"
+)
+
+var (
+	requestDuration = prometheus.NewHistogramVec(
+		prometheus.HistogramOpts{
+			Name:    "http_request_duration_seconds",
+			Help:    "Duration of HTTP requests.",
+			Buckets: prometheus.DefBuckets,
+		},
+		[]string{"path"},
+	)
+)
+
+func init() {
+	prometheus.MustRegister(requestDuration)
+}
+
+// MetricsHandler handles prometheus metrics requests for fasthttp
+func MetricsHandler(ctx *fasthttp.RequestCtx) {
+	ctx.SetContentType("text/plain; version=0.0.4; charset=utf-8")
+
+	mfs, err := prometheus.DefaultGatherer.Gather()
+	if err != nil {
+		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
+		ctx.SetBodyString("Failed to gather metrics: " + err.Error())
+		return
+	}
+
+	for _, mf := range mfs {
+		writeMetricFamily(ctx, mf)
+	}
+

[TRUNCATED OUTPUT]
```


> NOTE: Diff was truncated to 2000 lines. Consider splitting the PR.


## Command outputs


### `make test`

- Exit code: `0`


#### stdout

```text
🧪 Testing Subscription External Service...
	github.com/seidu626/subscription-manager/subscription-external/cmd		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/cmd/authkey		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/cmd/batch-processor		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/cmd/docs		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/cmd/notification-monitor		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/cmd/resubscribe-processor		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/docs		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/config		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/domain		coverage: 0.0% of statements
=== RUN   TestOptinHandler_ErrorHandling
=== RUN   TestOptinHandler_ErrorHandling/MTResponseError_-_INVALID_MSISDN
=== RUN   TestOptinHandler_ErrorHandling/MTResponseError_-_OPTIN_CONFIG_NOT_FOUND
=== RUN   TestOptinHandler_ErrorHandling/Generic_Error
--- PASS: TestOptinHandler_ErrorHandling (0.00s)
    --- PASS: TestOptinHandler_ErrorHandling/MTResponseError_-_INVALID_MSISDN (0.00s)
    --- PASS: TestOptinHandler_ErrorHandling/MTResponseError_-_OPTIN_CONFIG_NOT_FOUND (0.00s)
    --- PASS: TestOptinHandler_ErrorHandling/Generic_Error (0.00s)
PASS
coverage: 1.1% of statements
ok  	github.com/seidu626/subscription-manager/subscription-external/internal/handler	(cached)	coverage: 1.1% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/logging		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/middleware		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/monitoring		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/repository		coverage: 0.0% of statements
=== RUN   TestEnhancedBlacklistedUserHandling
2026-01-24T01:03:12.250Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-123", "partnerId": 789}
2026-01-24T01:03:12.250Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:12.250Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:12.250Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456789", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:123,partnerId:789,requestId:test-request-id-123"}
2026-01-24T01:03:12.250Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456789", "requestId": "test-request-id-123"}
2026-01-24T01:03:12.250Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-123", "duration": "213.713µs"}
--- PASS: TestEnhancedBlacklistedUserHandling (0.20s)
=== RUN   TestBlacklistedUserRetryLogic
2026-01-24T01:03:12.451Z	WARN	service/subscription.go:2773	Failed to add user to blacklist, retrying	{"msisdn": "233123456790", "attempt": 1, "maxRetries": 3, "error": "failed to insert blacklisted user: assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).addUserToBlacklistWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2773
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2733
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestBlacklistedUserRetryLogic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_blacklisted_test.go:168
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:12.551Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456790", "productId": 456, "requestId": "test-request-id-456", "partnerId": 790}
2026-01-24T01:03:12.551Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456790", "attempt": 2}
2026-01-24T01:03:12.551Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456790", "attempt": 1}
2026-01-24T01:03:12.552Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456790", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:456,partnerId:790,requestId:test-request-id-456"}
2026-01-24T01:03:12.552Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456790", "requestId": "test-request-id-456"}
2026-01-24T01:03:12.552Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456790", "productId": 456, "requestId": "test-request-id-456", "duration": "100.439519ms"}
--- PASS: TestBlacklistedUserRetryLogic (0.70s)
=== RUN   TestBatchBlacklistedUserProcessing
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2874	Starting batch processing of blacklisted users	{"totalResponses": 3, "totalRequests": 3}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2900	Processing blacklisted users in batch	{"blacklistedCount": 2}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456791", "productId": 125, "requestId": "test-request-id-3", "partnerId": 789}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456791", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:125,partnerId:789,requestId:test-request-id-3"}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456791", "requestId": "test-request-id-3"}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-1", "partnerId": 789}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456791", "productId": 125, "requestId": "test-request-id-3", "duration": "96.863µs"}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456789", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:123,partnerId:789,requestId:test-request-id-1"}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456789", "requestId": "test-request-id-1"}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-1", "duration": "142.868µs"}
2026-01-24T01:03:13.152Z	INFO	service/subscription.go:2928	Completed batch processing of blacklisted users	{"processedCount": 2, "duration": "287.419µs"}
--- PASS: TestBatchBlacklistedUserProcessing (0.30s)
=== RUN   TestBlacklistedUserAuditLogging
2026-01-24T01:03:13.453Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456792", "productId": 789, "requestId": "test-request-id-audit", "partnerId": 791}
2026-01-24T01:03:13.453Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456792", "attempt": 1}
2026-01-24T01:03:13.453Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456792", "attempt": 1}
2026-01-24T01:03:13.453Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456792", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:789,partnerId:791,requestId:test-request-id-audit"}
2026-01-24T01:03:13.453Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456792", "requestId": "test-request-id-audit"}
2026-01-24T01:03:13.453Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456792", "productId": 789, "requestId": "test-request-id-audit", "duration": "85.834µs"}
--- PASS: TestBlacklistedUserAuditLogging (0.10s)
=== RUN   TestBlacklistedUserMetrics
--- PASS: TestBlacklistedUserMetrics (0.00s)
=== RUN   TestBlacklistedUserErrorHandling
2026-01-24T01:03:13.554Z	WARN	service/subscription.go:2773	Failed to add user to blacklist, retrying	{"msisdn": "233123456793", "attempt": 1, "maxRetries": 3, "error": "failed to insert blacklisted user: assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).addUserToBlacklistWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2773
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2733
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestBlacklistedUserErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_blacklisted_test.go:377
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:13.654Z	WARN	service/subscription.go:2773	Failed to add user to blacklist, retrying	{"msisdn": "233123456793", "attempt": 2, "maxRetries": 3, "error": "failed to insert blacklisted user: assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).addUserToBlacklistWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2773
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2733
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestBlacklistedUserErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_blacklisted_test.go:377
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:14.054Z	WARN	service/subscription.go:2773	Failed to add user to blacklist, retrying	{"msisdn": "233123456793", "attempt": 3, "maxRetries": 3, "error": "failed to insert blacklisted user: assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).addUserToBlacklistWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2773
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2733
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestBlacklistedUserErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_blacklisted_test.go:377
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:14.055Z	ERROR	service/subscription.go:2734	Failed to add user to blacklist with retry	{"msisdn": "233123456793", "error": "failed to add user to blacklist after 3 retries"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2734
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestBlacklistedUserErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_blacklisted_test.go:377
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:14.055Z	ERROR	service/subscription.go:2724	Enhanced BLACKLISTED user processing failed	{"msisdn": "233123456793", "productId": 792, "requestId": "test-request-id-error", "duration": "501.051648ms"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2724
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleBlacklistedUserEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:2737
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestBlacklistedUserErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_blacklisted_test.go:377
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestBlacklistedUserErrorHandling (1.10s)
=== RUN   TestBlacklistedUserConfiguration
2026-01-24T01:03:14.655Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "partnerId": 789}
2026-01-24T01:03:14.655Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:14.655Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:14.655Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456789", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:123,partnerId:789,requestId:test-request-id"}
2026-01-24T01:03:14.655Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456789", "requestId": "test-request-id"}
2026-01-24T01:03:14.655Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "97.797µs"}
2026-01-24T01:03:14.756Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456790", "productId": 123, "requestId": "test-request-id", "partnerId": 789}
2026-01-24T01:03:14.756Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456790", "attempt": 1}
2026-01-24T01:03:14.756Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456790", "attempt": 1}
2026-01-24T01:03:14.756Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456790", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:123,partnerId:789,requestId:test-request-id"}
2026-01-24T01:03:14.756Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456790", "requestId": "test-request-id"}
2026-01-24T01:03:14.756Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456790", "productId": 123, "requestId": "test-request-id", "duration": "131.235µs"}
2026-01-24T01:03:14.857Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456791", "productId": 123, "requestId": "test-request-id", "partnerId": 789}
2026-01-24T01:03:14.857Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:14.857Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:14.857Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456791", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:123,partnerId:789,requestId:test-request-id"}
2026-01-24T01:03:14.857Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456791", "requestId": "test-request-id"}
2026-01-24T01:03:14.857Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456791", "productId": 123, "requestId": "test-request-id", "duration": "202.415µs"}
--- PASS: TestBlacklistedUserConfiguration (0.30s)
=== RUN   TestBlacklistedUserPerformance
2026-01-24T01:03:14.958Z	INFO	service/subscription.go:2874	Starting batch processing of blacklisted users	{"totalResponses": 50, "totalRequests": 50}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2900	Processing blacklisted users in batch	{"blacklistedCount": 50}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456049", "productId": 149, "requestId": "test-request-id-49", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456049", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456003", "productId": 103, "requestId": "test-request-id-3", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456003", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456049", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456004", "productId": 104, "requestId": "test-request-id-4", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456049", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:149,partnerId:789,requestId:test-request-id-49"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456049", "requestId": "test-request-id-49"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456049", "productId": 149, "requestId": "test-request-id-49", "duration": "317.487µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456004", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456036", "productId": 136, "requestId": "test-request-id-36", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456036", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456003", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456003", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:103,partnerId:789,requestId:test-request-id-3"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456003", "requestId": "test-request-id-3"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456003", "productId": 103, "requestId": "test-request-id-3", "duration": "342.895µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456036", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456036", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:136,partnerId:789,requestId:test-request-id-36"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456036", "requestId": "test-request-id-36"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456036", "productId": 136, "requestId": "test-request-id-36", "duration": "216.396µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456005", "productId": 105, "requestId": "test-request-id-5", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456004", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456005", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456004", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:104,partnerId:789,requestId:test-request-id-4"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456004", "requestId": "test-request-id-4"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456009", "productId": 109, "requestId": "test-request-id-9", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456009", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456004", "productId": 104, "requestId": "test-request-id-4", "duration": "346.003µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456005", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456005", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:105,partnerId:789,requestId:test-request-id-5"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456005", "requestId": "test-request-id-5"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456005", "productId": 105, "requestId": "test-request-id-5", "duration": "322.164µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456010", "productId": 110, "requestId": "test-request-id-10", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456010", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456010", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456010", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:110,partnerId:789,requestId:test-request-id-10"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456010", "requestId": "test-request-id-10"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456010", "productId": 110, "requestId": "test-request-id-10", "duration": "175.66µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456000", "productId": 100, "requestId": "test-request-id-0", "partnerId": 789}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456009", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456000", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456009", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:109,partnerId:789,requestId:test-request-id-9"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456009", "requestId": "test-request-id-9"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456009", "productId": 109, "requestId": "test-request-id-9", "duration": "370.742µs"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456000", "attempt": 1}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456000", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:100,partnerId:789,requestId:test-request-id-0"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456000", "requestId": "test-request-id-0"}
2026-01-24T01:03:14.959Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456000", "productId": 100, "requestId": "test-request-id-0", "duration": "841.106µs"}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456001", "productId": 101, "requestId": "test-request-id-1", "partnerId": 789}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456001", "attempt": 1}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456001", "attempt": 1}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456001", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:101,partnerId:789,requestId:test-request-id-1"}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456001", "requestId": "test-request-id-1"}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456001", "productId": 101, "requestId": "test-request-id-1", "duration": "1.567784ms"}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456002", "productId": 102, "requestId": "test-request-id-2", "partnerId": 789}
2026-01-24T01:03:14.960Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456002", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456002", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456006", "productId": 106, "requestId": "test-request-id-6", "partnerId": 789}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456002", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:102,partnerId:789,requestId:test-request-id-2"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456024", "productId": 124, "requestId": "test-request-id-24", "partnerId": 789}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456002", "requestId": "test-request-id-2"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456006", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456024", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456002", "productId": 102, "requestId": "test-request-id-2", "duration": "2.571668ms"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456006", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456006", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:106,partnerId:789,requestId:test-request-id-6"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456006", "requestId": "test-request-id-6"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456006", "productId": 106, "requestId": "test-request-id-6", "duration": "2.435926ms"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456007", "productId": 107, "requestId": "test-request-id-7", "partnerId": 789}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456007", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456007", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456017", "productId": 117, "requestId": "test-request-id-17", "partnerId": 789}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456007", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:107,partnerId:789,requestId:test-request-id-7"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456017", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456007", "requestId": "test-request-id-7"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456007", "productId": 107, "requestId": "test-request-id-7", "duration": "2.490757ms"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456017", "attempt": 1}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456017", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:117,partnerId:789,requestId:test-request-id-17"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456017", "requestId": "test-request-id-17"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456017", "productId": 117, "requestId": "test-request-id-17", "duration": "134.519µs"}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456008", "productId": 108, "requestId": "test-request-id-8", "partnerId": 789}
2026-01-24T01:03:14.961Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456008", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456019", "productId": 119, "requestId": "test-request-id-19", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456019", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456008", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456011", "productId": 111, "requestId": "test-request-id-11", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456008", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:108,partnerId:789,requestId:test-request-id-8"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456011", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456008", "requestId": "test-request-id-8"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456008", "productId": 108, "requestId": "test-request-id-8", "duration": "2.576749ms"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456011", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456011", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:111,partnerId:789,requestId:test-request-id-11"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456011", "requestId": "test-request-id-11"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456011", "productId": 111, "requestId": "test-request-id-11", "duration": "2.433563ms"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456024", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456024", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:124,partnerId:789,requestId:test-request-id-24"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456024", "requestId": "test-request-id-24"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456024", "productId": 124, "requestId": "test-request-id-24", "duration": "3.134827ms"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456019", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456019", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:119,partnerId:789,requestId:test-request-id-19"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456019", "requestId": "test-request-id-19"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456019", "productId": 119, "requestId": "test-request-id-19", "duration": "352.943µs"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456016", "productId": 116, "requestId": "test-request-id-16", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456016", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456016", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456018", "productId": 118, "requestId": "test-request-id-18", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456016", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:116,partnerId:789,requestId:test-request-id-16"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456018", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456016", "requestId": "test-request-id-16"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456016", "productId": 116, "requestId": "test-request-id-16", "duration": "820.071µs"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456018", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456018", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:118,partnerId:789,requestId:test-request-id-18"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456018", "requestId": "test-request-id-18"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456018", "productId": 118, "requestId": "test-request-id-18", "duration": "701.116µs"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456030", "productId": 130, "requestId": "test-request-id-30", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456030", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456030", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456030", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:130,partnerId:789,requestId:test-request-id-30"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456030", "requestId": "test-request-id-30"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456030", "productId": 130, "requestId": "test-request-id-30", "duration": "220.616µs"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456021", "productId": 121, "requestId": "test-request-id-21", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456021", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456026", "productId": 126, "requestId": "test-request-id-26", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456020", "productId": 120, "requestId": "test-request-id-20", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456021", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456020", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456026", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456021", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:121,partnerId:789,requestId:test-request-id-21"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456021", "requestId": "test-request-id-21"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456021", "productId": 121, "requestId": "test-request-id-21", "duration": "776.334µs"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456020", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456020", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:120,partnerId:789,requestId:test-request-id-20"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456027", "productId": 127, "requestId": "test-request-id-27", "partnerId": 789}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456020", "requestId": "test-request-id-20"}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456027", "attempt": 1}
2026-01-24T01:03:14.962Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456020", "productId": 120, "requestId": "test-request-id-20", "duration": "850.334µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456028", "productId": 128, "requestId": "test-request-id-28", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456027", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456028", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456027", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:127,partnerId:789,requestId:test-request-id-27"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456027", "requestId": "test-request-id-27"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456027", "productId": 127, "requestId": "test-request-id-27", "duration": "106.232µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456028", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456028", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:128,partnerId:789,requestId:test-request-id-28"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456022", "productId": 122, "requestId": "test-request-id-22", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456022", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456028", "requestId": "test-request-id-28"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456028", "productId": 128, "requestId": "test-request-id-28", "duration": "141.054µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456026", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456012", "productId": 112, "requestId": "test-request-id-12", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456026", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:126,partnerId:789,requestId:test-request-id-26"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456012", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456026", "requestId": "test-request-id-26"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456026", "productId": 126, "requestId": "test-request-id-26", "duration": "359.079µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456012", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456012", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:112,partnerId:789,requestId:test-request-id-12"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456031", "productId": 131, "requestId": "test-request-id-31", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456013", "productId": 113, "requestId": "test-request-id-13", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456031", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456013", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456014", "productId": 114, "requestId": "test-request-id-14", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456014", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456033", "productId": 133, "requestId": "test-request-id-33", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456031", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456015", "productId": 115, "requestId": "test-request-id-15", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456031", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:131,partnerId:789,requestId:test-request-id-31"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456015", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456031", "requestId": "test-request-id-31"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456031", "productId": 131, "requestId": "test-request-id-31", "duration": "109.362µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456033", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456012", "requestId": "test-request-id-12"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456015", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456015", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:115,partnerId:789,requestId:test-request-id-15"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456015", "requestId": "test-request-id-15"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456015", "productId": 115, "requestId": "test-request-id-15", "duration": "2.583987ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456012", "productId": 112, "requestId": "test-request-id-12", "duration": "3.503476ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456023", "productId": 123, "requestId": "test-request-id-23", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456023", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456013", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456013", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:113,partnerId:789,requestId:test-request-id-13"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456013", "requestId": "test-request-id-13"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456013", "productId": 113, "requestId": "test-request-id-13", "duration": "3.437697ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456025", "productId": 125, "requestId": "test-request-id-25", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456025", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456023", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456023", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:123,partnerId:789,requestId:test-request-id-23"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456023", "requestId": "test-request-id-23"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456033", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456023", "productId": 123, "requestId": "test-request-id-23", "duration": "1.139849ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456033", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:133,partnerId:789,requestId:test-request-id-33"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456032", "productId": 132, "requestId": "test-request-id-32", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456033", "requestId": "test-request-id-33"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456032", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456033", "productId": 133, "requestId": "test-request-id-33", "duration": "351.091µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456029", "productId": 129, "requestId": "test-request-id-29", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456029", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456014", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456034", "productId": 134, "requestId": "test-request-id-34", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456034", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456014", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:114,partnerId:789,requestId:test-request-id-14"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456014", "requestId": "test-request-id-14"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456014", "productId": 114, "requestId": "test-request-id-14", "duration": "3.568565ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456034", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456034", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:134,partnerId:789,requestId:test-request-id-34"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456034", "requestId": "test-request-id-34"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456034", "productId": 134, "requestId": "test-request-id-34", "duration": "274.753µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456039", "productId": 139, "requestId": "test-request-id-39", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456039", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456022", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456022", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:122,partnerId:789,requestId:test-request-id-22"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456022", "requestId": "test-request-id-22"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456022", "productId": 122, "requestId": "test-request-id-22", "duration": "1.452471ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456025", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456025", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:125,partnerId:789,requestId:test-request-id-25"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456025", "requestId": "test-request-id-25"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456025", "productId": 125, "requestId": "test-request-id-25", "duration": "1.149025ms"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456043", "productId": 143, "requestId": "test-request-id-43", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456032", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456032", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:132,partnerId:789,requestId:test-request-id-32"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456043", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456032", "requestId": "test-request-id-32"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456032", "productId": 132, "requestId": "test-request-id-32", "duration": "522.279µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456047", "productId": 147, "requestId": "test-request-id-47", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456047", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456040", "productId": 140, "requestId": "test-request-id-40", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456040", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456037", "productId": 137, "requestId": "test-request-id-37", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456037", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456043", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456043", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:143,partnerId:789,requestId:test-request-id-43"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456038", "productId": 138, "requestId": "test-request-id-38", "partnerId": 789}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456038", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456043", "requestId": "test-request-id-43"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456043", "productId": 143, "requestId": "test-request-id-43", "duration": "536.278µs"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456038", "attempt": 1}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456038", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:138,partnerId:789,requestId:test-request-id-38"}
2026-01-24T01:03:14.963Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456038", "requestId": "test-request-id-38"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456038", "productId": 138, "requestId": "test-request-id-38", "duration": "505.924µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456044", "productId": 144, "requestId": "test-request-id-44", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456044", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456045", "productId": 145, "requestId": "test-request-id-45", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456045", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456047", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456047", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:147,partnerId:789,requestId:test-request-id-47"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456047", "requestId": "test-request-id-47"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456047", "productId": 147, "requestId": "test-request-id-47", "duration": "210.081µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456035", "productId": 135, "requestId": "test-request-id-35", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456035", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456045", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456045", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:145,partnerId:789,requestId:test-request-id-45"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456046", "productId": 146, "requestId": "test-request-id-46", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456045", "requestId": "test-request-id-45"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456046", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456045", "productId": 145, "requestId": "test-request-id-45", "duration": "94.885µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456041", "productId": 141, "requestId": "test-request-id-41", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456041", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456035", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456042", "productId": 142, "requestId": "test-request-id-42", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456035", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:135,partnerId:789,requestId:test-request-id-35"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456035", "requestId": "test-request-id-35"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456035", "productId": 135, "requestId": "test-request-id-35", "duration": "799.755µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456042", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2803	Successfully added user to blacklist (enhanced)	{"msisdn": "233123456048", "productId": 148, "requestId": "test-request-id-48", "partnerId": 789}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2767	Successfully added user to blacklist	{"msisdn": "233123456048", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456042", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456042", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:142,partnerId:789,requestId:test-request-id-42"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456042", "requestId": "test-request-id-42"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456042", "productId": 142, "requestId": "test-request-id-42", "duration": "490.726µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456048", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456048", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:148,partnerId:789,requestId:test-request-id-48"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456048", "requestId": "test-request-id-48"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456048", "productId": 148, "requestId": "test-request-id-48", "duration": "277.606µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456029", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456029", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:129,partnerId:789,requestId:test-request-id-29"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456029", "requestId": "test-request-id-29"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456029", "productId": 129, "requestId": "test-request-id-29", "duration": "1.416011ms"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456040", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456040", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:140,partnerId:789,requestId:test-request-id-40"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456040", "requestId": "test-request-id-40"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456040", "productId": 140, "requestId": "test-request-id-40", "duration": "915.259µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456037", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456037", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:137,partnerId:789,requestId:test-request-id-37"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456037", "requestId": "test-request-id-37"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456037", "productId": 137, "requestId": "test-request-id-37", "duration": "1.167679ms"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456044", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456044", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:144,partnerId:789,requestId:test-request-id-44"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456044", "requestId": "test-request-id-44"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456044", "productId": 144, "requestId": "test-request-id-44", "duration": "805.774µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456046", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456046", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:146,partnerId:789,requestId:test-request-id-46"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456046", "requestId": "test-request-id-46"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456046", "productId": 146, "requestId": "test-request-id-46", "duration": "756.91µs"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456041", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456041", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:141,partnerId:789,requestId:test-request-id-41"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456041", "requestId": "test-request-id-41"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456041", "productId": 141, "requestId": "test-request-id-41", "duration": "1.160835ms"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2817	Successfully removed user subscriptions	{"msisdn": "233123456039", "attempt": 1}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2857	Blacklisted user audit log entry created	{"msisdn": "233123456039", "action": "USER_BLACKLISTED", "reason": "MT Response indicated BLACKLISTED status", "metadata": "productId:139,partnerId:789,requestId:test-request-id-39"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2757	Successfully processed enhanced BLACKLISTED user	{"msisdn": "233123456039", "requestId": "test-request-id-39"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2718	Enhanced BLACKLISTED user processing completed successfully	{"msisdn": "233123456039", "productId": 139, "requestId": "test-request-id-39", "duration": "1.333456ms"}
2026-01-24T01:03:14.964Z	INFO	service/subscription.go:2928	Completed batch processing of blacklisted users	{"processedCount": 50, "duration": "5.985555ms"}
--- PASS: TestBlacklistedUserPerformance (0.51s)
=== RUN   TestIntegrationInvalidMSISDNHandling
2026-01-24T01:03:15.466Z	WARN	service/subscription.go:1246	INVALID_MSISDN detected, logging for reference and cleaning up subscriptions	{"msisdn": "233123456789", "responseCode": "INVALID_MSISDN", "subscriptionResult": "INVALID_MSISDN", "subscriptionError": "null"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).detectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1246
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationInvalidMSISDNHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:295
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:15.466Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:15.466Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-123", "duration": "43.887µs"}
--- PASS: TestIntegrationInvalidMSISDNHandling (0.20s)
=== RUN   TestIntegrationProductIndependentCleanup
2026-01-24T01:03:15.666Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456790", "attempt": 1}
2026-01-24T01:03:15.666Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456790", "productId": 456, "requestId": "test-request-id-456", "duration": "43.146µs"}
--- PASS: TestIntegrationProductIndependentCleanup (0.10s)
=== RUN   TestIntegrationBatchProcessing
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1370	Starting batch processing of INVALID_MSISDN responses	{"responseCount": 3, "requestCount": 3}
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1431	Found INVALID_MSISDN responses in batch, processing cleanup	{"invalidCount": 2, "invalidMSISDNs": ["233123456789", "233123456791"]}
2026-01-24T01:03:15.767Z	DEBUG	service/subscription.go:1520	Processed batch of invalid MSISDN logs	{"batchStart": 0, "batchEnd": 2, "batchSize": 2}
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456791", "productId": 125, "requestId": "test-request-id-3", "duration": "123.689µs"}
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-1", "duration": "226.622µs"}
2026-01-24T01:03:15.767Z	INFO	service/subscription.go:1561	Completed batch cleanup of invalid MSISDN subscriptions	{"totalTasks": 2}
--- PASS: TestIntegrationBatchProcessing (0.30s)
=== RUN   TestIntegrationMetricsAndMonitoring
2026-01-24T01:03:16.068Z	ERROR	monitoring/invalid_msisdn_metrics.go:106	INVALID_MSISDN cleanup failure recorded	{"errorType": "database_error", "error": "assert.AnError general error for testing", "totalFailures": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/monitoring.(*InvalidMSISDNMetrics).RecordCleanupFailure
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/monitoring/invalid_msisdn_metrics.go:106
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationMetricsAndMonitoring
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:437
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.068Z	INFO	monitoring/invalid_msisdn_metrics.go:192	INVALID_MSISDN metrics reset
--- PASS: TestIntegrationMetricsAndMonitoring (0.00s)
=== RUN   TestIntegrationErrorHandling
2026-01-24T01:03:16.068Z	ERROR	service/subscription.go:1305	Failed to check subscription existence for cleanup	{"msisdn": "233123456792", "error": "failed to check subscription existence: assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1305
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:480
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.068Z	ERROR	service/subscription.go:1294	Invalid MSISDN cleanup failed	{"msisdn": "233123456792", "productId": 789, "requestId": "test-request-id-error", "duration": "27.488µs"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1294
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1308
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:480
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.169Z	WARN	service/subscription.go:1331	Failed to delete subscription records for invalid MSISDN, retrying	{"msisdn": "233123456793", "attempt": 1, "maxRetries": 3, "error": "assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1331
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:498
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.269Z	WARN	service/subscription.go:1331	Failed to delete subscription records for invalid MSISDN, retrying	{"msisdn": "233123456793", "attempt": 2, "maxRetries": 3, "error": "assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1331
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:498
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.670Z	WARN	service/subscription.go:1331	Failed to delete subscription records for invalid MSISDN, retrying	{"msisdn": "233123456793", "attempt": 3, "maxRetries": 3, "error": "assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1331
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:498
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.670Z	ERROR	service/subscription.go:1346	Failed to delete subscription records for invalid MSISDN after all retries	{"msisdn": "233123456793", "maxRetries": 3, "error": "assert.AnError general error for testing"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1346
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:498
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.670Z	ERROR	service/subscription.go:1294	Invalid MSISDN cleanup failed	{"msisdn": "233123456793", "productId": 790, "requestId": "test-request-id-retry", "duration": "500.999142ms"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1294
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1351
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestIntegrationErrorHandling
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_integration_test.go:498
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestIntegrationErrorHandling (1.20s)
=== RUN   TestIntegrationConfiguration
2026-01-24T01:03:17.271Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:17.271Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "141.334µs"}
2026-01-24T01:03:17.371Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456790", "attempt": 1}
2026-01-24T01:03:17.371Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456790", "productId": 123, "requestId": "test-request-id", "duration": "81.995µs"}
2026-01-24T01:03:17.472Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:17.472Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456791", "productId": 123, "requestId": "test-request-id", "duration": "126.484µs"}
--- PASS: TestIntegrationConfiguration (0.30s)
=== RUN   TestIntegrationPerformance
2026-01-24T01:03:17.573Z	INFO	service/subscription.go:1370	Starting batch processing of INVALID_MSISDN responses	{"responseCount": 100, "requestCount": 100}
2026-01-24T01:03:17.574Z	INFO	service/subscription.go:1431	Found INVALID_MSISDN responses in batch, processing cleanup	{"invalidCount": 100, "invalidMSISDNs": ["233123456000", "233123456001", "233123456002", "233123456003", "233123456004", "233123456005", "233123456006", "233123456007", "233123456008", "233123456009", "233123456010", "233123456011", "233123456012", "233123456013", "233123456014", "233123456015", "233123456016", "233123456017", "233123456018", "233123456019", "233123456020", "233123456021", "233123456022", "233123456023", "233123456024", "233123456025", "233123456026", "233123456027", "233123456028", "233123456029", "233123456030", "233123456031", "233123456032", "233123456033", "233123456034", "233123456035", "233123456036", "233123456037", "233123456038", "233123456039", "233123456040", "233123456041", "233123456042", "233123456043", "233123456044", "233123456045", "233123456046", "233123456047", "233123456048", "233123456049", "233123456050", "233123456051", "233123456052", "233123456053", "233123456054", "233123456055", "233123456056", "233123456057", "233123456058", "233123456059", "233123456060", "233123456061", "233123456062", "233123456063", "233123456064", "233123456065", "233123456066", "233123456067", "233123456068", "233123456069", "233123456070", "233123456071", "233123456072", "233123456073", "233123456074", "233123456075", "233123456076", "233123456077", "233123456078", "233123456079", "233123456080", "233123456081", "233123456082", "233123456083", "233123456084", "233123456085", "233123456086", "233123456087", "233123456088", "233123456089", "233123456090", "233123456091", "233123456092", "233123456093", "233123456094", "233123456095", "233123456096", "233123456097", "233123456098", "233123456099"]}
2026-01-24T01:03:17.575Z	DEBUG	service/subscription.go:1520	Processed batch of invalid MSISDN logs	{"batchStart": 0, "batchEnd": 100, "batchSize": 100}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456005", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456005", "productId": 105, "requestId": "test-request-id-5", "duration": "72.978µs"}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456014", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456014", "productId": 114, "requestId": "test-request-id-14", "duration": "60.55µs"}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456015", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456015", "productId": 115, "requestId": "test-request-id-15", "duration": "57.537µs"}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456099", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456099", "productId": 199, "requestId": "test-request-id-99", "duration": "327.361µs"}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456006", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456006", "productId": 106, "requestId": "test-request-id-6", "duration": "382.919µs"}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456018", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456018", "productId": 118, "requestId": "test-request-id-18", "duration": "46.272µs"}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456007", "attempt": 1}
2026-01-24T01:03:17.575Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456007", "productId": 107, "requestId": "test-request-id-7", "duration": "489.469µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456020", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456020", "productId": 120, "requestId": "test-request-id-20", "duration": "46.741µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456021", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456021", "productId": 121, "requestId": "test-request-id-21", "duration": "43.015µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456022", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456022", "productId": 122, "requestId": "test-request-id-22", "duration": "42.962µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456023", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456023", "productId": 123, "requestId": "test-request-id-23", "duration": "44.102µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456024", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456024", "productId": 124, "requestId": "test-request-id-24", "duration": "43.155µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456025", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456025", "productId": 125, "requestId": "test-request-id-25", "duration": "45.518µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456026", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456026", "productId": 126, "requestId": "test-request-id-26", "duration": "46.176µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456027", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456027", "productId": 127, "requestId": "test-request-id-27", "duration": "45.83µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456028", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456028", "productId": 128, "requestId": "test-request-id-28", "duration": "47.45µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456029", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456029", "productId": 129, "requestId": "test-request-id-29", "duration": "48.684µs"}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456030", "attempt": 1}
2026-01-24T01:03:17.576Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456030", "productId": 130, "requestId": "test-request-id-30", "duration": "47.604µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456031", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456031", "productId": 131, "requestId": "test-request-id-31", "duration": "548.425µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456032", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456032", "productId": 132, "requestId": "test-request-id-32", "duration": "52.174µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456033", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456033", "productId": 133, "requestId": "test-request-id-33", "duration": "49.486µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456034", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456034", "productId": 134, "requestId": "test-request-id-34", "duration": "52.663µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456035", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456035", "productId": 135, "requestId": "test-request-id-35", "duration": "53.553µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456036", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456036", "productId": 136, "requestId": "test-request-id-36", "duration": "56.632µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456037", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456037", "productId": 137, "requestId": "test-request-id-37", "duration": "60.003µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456038", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456038", "productId": 138, "requestId": "test-request-id-38", "duration": "61.653µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456039", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456039", "productId": 139, "requestId": "test-request-id-39", "duration": "78.255µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456019", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456019", "productId": 119, "requestId": "test-request-id-19", "duration": "1.688124ms"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456041", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456041", "productId": 141, "requestId": "test-request-id-41", "duration": "120.825µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456042", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456042", "productId": 142, "requestId": "test-request-id-42", "duration": "109.702µs"}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456008", "attempt": 1}
2026-01-24T01:03:17.577Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456008", "productId": 108, "requestId": "test-request-id-8", "duration": "2.423066ms"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456012", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456012", "productId": 112, "requestId": "test-request-id-12", "duration": "2.503174ms"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456045", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456045", "productId": 145, "requestId": "test-request-id-45", "duration": "89.928µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456046", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456046", "productId": 146, "requestId": "test-request-id-46", "duration": "76.631µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456047", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456047", "productId": 147, "requestId": "test-request-id-47", "duration": "68.013µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456048", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456048", "productId": 148, "requestId": "test-request-id-48", "duration": "72.368µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456049", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456049", "productId": 149, "requestId": "test-request-id-49", "duration": "70.512µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456050", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456050", "productId": 150, "requestId": "test-request-id-50", "duration": "70.03µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456051", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456051", "productId": 151, "requestId": "test-request-id-51", "duration": "70.613µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456052", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456052", "productId": 152, "requestId": "test-request-id-52", "duration": "70.307µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456053", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456053", "productId": 153, "requestId": "test-request-id-53", "duration": "70.655µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456054", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456054", "productId": 154, "requestId": "test-request-id-54", "duration": "71.916µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456055", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456055", "productId": 155, "requestId": "test-request-id-55", "duration": "127.15µs"}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456009", "attempt": 1}
2026-01-24T01:03:17.578Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456009", "productId": 109, "requestId": "test-request-id-9", "duration": "3.467796ms"}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456016", "attempt": 1}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456016", "productId": 116, "requestId": "test-request-id-16", "duration": "3.374453ms"}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456017", "attempt": 1}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456017", "productId": 117, "requestId": "test-request-id-17", "duration": "3.311469ms"}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456040", "attempt": 1}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456040", "productId": 140, "requestId": "test-request-id-40", "duration": "1.744374ms"}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456056", "attempt": 1}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456056", "productId": 156, "requestId": "test-request-id-56", "duration": "591.658µs"}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456061", "attempt": 1}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456061", "productId": 161, "requestId": "test-request-id-61", "duration": "223.216µs"}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456062", "attempt": 1}
2026-01-24T01:03:17.579Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456062", "productId": 162, "requestId": "test-request-id-62", "duration": "234.46µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456063", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456063", "productId": 163, "requestId": "test-request-id-63", "duration": "137.385µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456064", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456064", "productId": 164, "requestId": "test-request-id-64", "duration": "288.632µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456065", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456065", "productId": 165, "requestId": "test-request-id-65", "duration": "88.878µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456066", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456066", "productId": 166, "requestId": "test-request-id-66", "duration": "92.628µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456067", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456067", "productId": 167, "requestId": "test-request-id-67", "duration": "88.676µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456068", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456068", "productId": 168, "requestId": "test-request-id-68", "duration": "90.014µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456069", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456069", "productId": 169, "requestId": "test-request-id-69", "duration": "92.047µs"}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456070", "attempt": 1}
2026-01-24T01:03:17.580Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456070", "productId": 170, "requestId": "test-request-id-70", "duration": "91.4µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456071", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456071", "productId": 171, "requestId": "test-request-id-71", "duration": "104.056µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456072", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456072", "productId": 172, "requestId": "test-request-id-72", "duration": "110.597µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456060", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456060", "productId": 160, "requestId": "test-request-id-60", "duration": "1.977901ms"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456044", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456044", "productId": 144, "requestId": "test-request-id-44", "duration": "3.42831ms"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456043", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456043", "productId": 143, "requestId": "test-request-id-43", "duration": "3.520082ms"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456010", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456010", "productId": 110, "requestId": "test-request-id-10", "duration": "5.934265ms"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456077", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456077", "productId": 177, "requestId": "test-request-id-77", "duration": "106.868µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456078", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456078", "productId": 178, "requestId": "test-request-id-78", "duration": "105.762µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456079", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456079", "productId": 179, "requestId": "test-request-id-79", "duration": "99.618µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456080", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456080", "productId": 180, "requestId": "test-request-id-80", "duration": "103.014µs"}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456081", "attempt": 1}
2026-01-24T01:03:17.581Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456081", "productId": 181, "requestId": "test-request-id-81", "duration": "99.833µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456082", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456082", "productId": 182, "requestId": "test-request-id-82", "duration": "97.12µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456083", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456083", "productId": 183, "requestId": "test-request-id-83", "duration": "100.102µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456084", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456084", "productId": 184, "requestId": "test-request-id-84", "duration": "99.569µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456085", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456085", "productId": 185, "requestId": "test-request-id-85", "duration": "102.81µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456086", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456086", "productId": 186, "requestId": "test-request-id-86", "duration": "262.464µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456087", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456087", "productId": 187, "requestId": "test-request-id-87", "duration": "122.905µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456088", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456088", "productId": 188, "requestId": "test-request-id-88", "duration": "102.73µs"}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456089", "attempt": 1}
2026-01-24T01:03:17.582Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456089", "productId": 189, "requestId": "test-request-id-89", "duration": "104.99µs"}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456090", "attempt": 1}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456090", "productId": 190, "requestId": "test-request-id-90", "duration": "104.744µs"}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456091", "attempt": 1}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456091", "productId": 191, "requestId": "test-request-id-91", "duration": "105.255µs"}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456092", "attempt": 1}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456092", "productId": 192, "requestId": "test-request-id-92", "duration": "346.355µs"}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456075", "attempt": 1}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456075", "productId": 175, "requestId": "test-request-id-75", "duration": "2.387038ms"}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456094", "attempt": 1}
2026-01-24T01:03:17.583Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456094", "productId": 194, "requestId": "test-request-id-94", "duration": "175.465µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456095", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456095", "productId": 195, "requestId": "test-request-id-95", "duration": "150.628µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456096", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456096", "productId": 196, "requestId": "test-request-id-96", "duration": "150.744µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456097", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456097", "productId": 197, "requestId": "test-request-id-97", "duration": "151.453µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456098", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456098", "productId": 198, "requestId": "test-request-id-98", "duration": "161.438µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456002", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456002", "productId": 102, "requestId": "test-request-id-2", "duration": "43.009µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456093", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456093", "productId": 193, "requestId": "test-request-id-93", "duration": "1.222438ms"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456059", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456059", "productId": 159, "requestId": "test-request-id-59", "duration": "5.678457ms"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456057", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456057", "productId": 157, "requestId": "test-request-id-57", "duration": "5.912351ms"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456000", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456000", "productId": 100, "requestId": "test-request-id-0", "duration": "288.274µs"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456013", "attempt": 1}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456013", "productId": 113, "requestId": "test-request-id-13", "duration": "9.437328ms"}
2026-01-24T01:03:17.584Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456074", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456074", "productId": 174, "requestId": "test-request-id-74", "duration": "3.701729ms"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456003", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456003", "productId": 103, "requestId": "test-request-id-3", "duration": "214.343µs"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456076", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456076", "productId": 176, "requestId": "test-request-id-76", "duration": "3.694169ms"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456073", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456073", "productId": 173, "requestId": "test-request-id-73", "duration": "3.94686ms"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456001", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456001", "productId": 101, "requestId": "test-request-id-1", "duration": "394.583µs"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456058", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456058", "productId": 158, "requestId": "test-request-id-58", "duration": "6.138869ms"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456004", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456004", "productId": 104, "requestId": "test-request-id-4", "duration": "353.421µs"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456011", "attempt": 1}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456011", "productId": 111, "requestId": "test-request-id-11", "duration": "9.756611ms"}
2026-01-24T01:03:17.585Z	INFO	service/subscription.go:1561	Completed batch cleanup of invalid MSISDN subscriptions	{"totalTasks": 100}
--- PASS: TestIntegrationPerformance (0.52s)
=== RUN   TestDetectAndLogInvalidMSISDNEnhanced
2026-01-24T01:03:18.095Z	WARN	service/subscription.go:1246	INVALID_MSISDN detected, logging for reference and cleaning up subscriptions	{"msisdn": "233123456789", "responseCode": "INVALID_MSISDN", "subscriptionResult": "null", "subscriptionError": "null"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).detectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1246
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestDetectAndLogInvalidMSISDNEnhanced
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:287
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:18.095Z	DEBUG	service/subscription.go:1312	No subscriptions found for invalid MSISDN, skipping cleanup	{"msisdn": "233123456789"}
2026-01-24T01:03:18.095Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "21.75µs"}
--- PASS: TestDetectAndLogInvalidMSISDNEnhanced (0.10s)
=== RUN   TestHandleInvalidMSISDNCleanup
=== RUN   TestHandleInvalidMSISDNCleanup/subscription_exists_and_deletion_succeeds
2026-01-24T01:03:18.196Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:18.196Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "61.646µs"}
=== RUN   TestHandleInvalidMSISDNCleanup/subscription_does_not_exist
2026-01-24T01:03:18.296Z	DEBUG	service/subscription.go:1312	No subscriptions found for invalid MSISDN, skipping cleanup	{"msisdn": "233123456789"}
2026-01-24T01:03:18.296Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "59.685µs"}
=== RUN   TestHandleInvalidMSISDNCleanup/deletion_fails_multiple_times
2026-01-24T01:03:18.397Z	WARN	service/subscription.go:1331	Failed to delete subscription records for invalid MSISDN, retrying	{"msisdn": "233123456789", "attempt": 1, "maxRetries": 3, "error": "database error"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1331
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleInvalidMSISDNCleanup.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:351
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:18.497Z	WARN	service/subscription.go:1331	Failed to delete subscription records for invalid MSISDN, retrying	{"msisdn": "233123456789", "attempt": 2, "maxRetries": 3, "error": "database error"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1331
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleInvalidMSISDNCleanup.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:351
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:18.898Z	WARN	service/subscription.go:1331	Failed to delete subscription records for invalid MSISDN, retrying	{"msisdn": "233123456789", "attempt": 3, "maxRetries": 3, "error": "database error"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1331
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleInvalidMSISDNCleanup.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:351
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:18.898Z	ERROR	service/subscription.go:1346	Failed to delete subscription records for invalid MSISDN after all retries	{"msisdn": "233123456789", "maxRetries": 3, "error": "database error"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1346
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleInvalidMSISDNCleanup.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:351
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:18.898Z	ERROR	service/subscription.go:1294	Invalid MSISDN cleanup failed	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "501.093663ms"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1294
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).handleInvalidMSISDNCleanup
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1351
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleInvalidMSISDNCleanup.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:351
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestHandleInvalidMSISDNCleanup (1.20s)
    --- PASS: TestHandleInvalidMSISDNCleanup/subscription_exists_and_deletion_succeeds (0.10s)
    --- PASS: TestHandleInvalidMSISDNCleanup/subscription_does_not_exist (0.10s)
    --- PASS: TestHandleInvalidMSISDNCleanup/deletion_fails_multiple_times (1.00s)
=== RUN   TestBatchHandleInvalidMSISDNs
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1370	Starting batch processing of INVALID_MSISDN responses	{"responseCount": 3, "requestCount": 3}
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1431	Found INVALID_MSISDN responses in batch, processing cleanup	{"invalidCount": 2, "invalidMSISDNs": ["233123456789", "233123456791"]}
2026-01-24T01:03:19.399Z	DEBUG	service/subscription.go:1520	Processed batch of invalid MSISDN logs	{"batchStart": 0, "batchEnd": 2, "batchSize": 2}
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-1", "duration": "122.073µs"}
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456791", "attempt": 1}
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456791", "productId": 125, "requestId": "test-request-id-3", "duration": "164.025µs"}
2026-01-24T01:03:19.399Z	INFO	service/subscription.go:1561	Completed batch cleanup of invalid MSISDN subscriptions	{"totalTasks": 2}
--- PASS: TestBatchHandleInvalidMSISDNs (0.20s)
=== RUN   TestIsInvalidMSISDNResponse
--- PASS: TestIsInvalidMSISDNResponse (0.00s)
=== RUN   TestExtractSubscriptionResult
--- PASS: TestExtractSubscriptionResult (0.00s)
=== RUN   TestExtractSubscriptionError
--- PASS: TestExtractSubscriptionError (0.00s)
=== RUN   TestBatchCreateInvalidMSISDNLogs
2026-01-24T01:03:19.600Z	DEBUG	service/subscription.go:1520	Processed batch of invalid MSISDN logs	{"batchStart": 0, "batchEnd": 2, "batchSize": 2}
--- PASS: TestBatchCreateInvalidMSISDNLogs (0.10s)
=== RUN   TestBatchCleanupInvalidMSISDNSubscriptions
2026-01-24T01:03:19.700Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456790", "attempt": 1}
2026-01-24T01:03:19.700Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456790", "productId": 124, "requestId": "test-request-id-2", "duration": "98.326µs"}
2026-01-24T01:03:19.700Z	INFO	service/subscription.go:1324	Successfully deleted all subscription records for invalid MSISDN	{"msisdn": "233123456789", "attempt": 1}
2026-01-24T01:03:19.700Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-1", "duration": "115.821µs"}
2026-01-24T01:03:19.700Z	INFO	service/subscription.go:1561	Completed batch cleanup of invalid MSISDN subscriptions	{"totalTasks": 2}
--- PASS: TestBatchCleanupInvalidMSISDNSubscriptions (0.20s)
=== RUN   TestInvalidMSISDNMetrics
2026-01-24T01:03:19.901Z	ERROR	monitoring/invalid_msisdn_metrics.go:106	INVALID_MSISDN cleanup failure recorded	{"errorType": "database_error", "error": "connection failed", "totalFailures": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/monitoring.(*InvalidMSISDNMetrics).RecordCleanupFailure
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/monitoring/invalid_msisdn_metrics.go:106
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestInvalidMSISDNMetrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_invalid_msisdn_test.go:640
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:19.901Z	INFO	monitoring/invalid_msisdn_metrics.go:192	INVALID_MSISDN metrics reset
--- PASS: TestInvalidMSISDNMetrics (0.00s)
=== RUN   TestStaffCheckOnly
=== RUN   TestStaffCheckOnly/Staff_MSISDN_should_be_excluded
=== RUN   TestStaffCheckOnly/Non-Staff_MSISDN_should_not_be_excluded
=== RUN   TestStaffCheckOnly/Staff_check_error_should_be_handled
--- PASS: TestStaffCheckOnly (0.00s)
    --- PASS: TestStaffCheckOnly/Staff_MSISDN_should_be_excluded (0.00s)
    --- PASS: TestStaffCheckOnly/Non-Staff_MSISDN_should_not_be_excluded (0.00s)
    --- PASS: TestStaffCheckOnly/Staff_check_error_should_be_handled (0.00s)
=== RUN   TestStaffCheckIntegration
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:254	MSISDN is excluded type (Staff/Premier/Blacklisted), excluding from optin processing	{"msisdn": "233123456789"}
--- PASS: TestStaffCheckIntegration (0.00s)
=== RUN   TestHandleAlreadyActiveSubscription
=== RUN   TestHandleAlreadyActiveSubscription/Subscription_exists,_renewal_exists_-_should_skip
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1595	Handling already active subscription	{"msisdn": "233123456789", "productId": "123"}
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1676	Renewal notification already sent this month, skipping	{"msisdn": "233123456789", "productId": "123"}
=== RUN   TestHandleAlreadyActiveSubscription/Subscription_check_error
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1595	Handling already active subscription	{"msisdn": "233123456789", "productId": "123"}
2026-01-24T01:03:19.901Z	ERROR	service/subscription.go:1608	Failed to check subscription existence	{"error": "database error"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).HandleAlreadyActiveSubscription
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1608
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleAlreadyActiveSubscription.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_test.go:440
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
=== RUN   TestHandleAlreadyActiveSubscription/Renewal_check_error
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1595	Handling already active subscription	{"msisdn": "233123456789", "productId": "123"}
2026-01-24T01:03:19.901Z	ERROR	service/subscription.go:1657	Failed to check renewal notification existence	{"error": "database error"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).HandleAlreadyActiveSubscription
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1657
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestHandleAlreadyActiveSubscription.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_test.go:440
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestHandleAlreadyActiveSubscription (0.00s)
    --- PASS: TestHandleAlreadyActiveSubscription/Subscription_exists,_renewal_exists_-_should_skip (0.00s)
    --- PASS: TestHandleAlreadyActiveSubscription/Subscription_check_error (0.00s)
    --- PASS: TestHandleAlreadyActiveSubscription/Renewal_check_error (0.00s)
=== RUN   TestDetectAndLogInvalidMSISDN
2026-01-24T01:03:19.901Z	WARN	service/subscription.go:1246	INVALID_MSISDN detected, logging for reference and cleaning up subscriptions	{"msisdn": "233123456789", "responseCode": "INVALID_MSISDN", "subscriptionResult": "null", "subscriptionError": "null"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).detectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1246
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestDetectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_test.go:494
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:19.901Z	WARN	service/subscription.go:1246	INVALID_MSISDN detected, logging for reference and cleaning up subscriptions	{"msisdn": "233123456789", "responseCode": "SUCCESS", "subscriptionResult": "INVALID_MSISDN", "subscriptionError": "null"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).detectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1246
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestDetectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_test.go:508
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:19.901Z	WARN	service/subscription.go:1246	INVALID_MSISDN detected, logging for reference and cleaning up subscriptions	{"msisdn": "233123456789", "responseCode": "SUCCESS", "subscriptionResult": "null", "subscriptionError": "Invalid MSISDN"}
github.com/seidu626/subscription-manager/subscription-external/internal/service.(*SubscriptionService).detectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription.go:1246
github.com/seidu626/subscription-manager/subscription-external/internal/service.TestDetectAndLogInvalidMSISDN
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/service/subscription_test.go:522
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestDetectAndLogInvalidMSISDN (0.00s)
=== RUN   TestDeleteSubscriptionRecordForInvalidMSISDN
    subscription_test.go:578: Test completed successfully - subscription deletion functionality is working
--- PASS: TestDeleteSubscriptionRecordForInvalidMSISDN (0.00s)
=== RUN   TestShouldRetryWithSMS
=== RUN   TestShouldRetryWithSMS/Should_retry_-_OPTIN_CONFIG_NOT_FOUND_in_subscriptionResult
=== RUN   TestShouldRetryWithSMS/Should_retry_-_OPTIN_CONFIG_NOT_FOUND_in_subscriptionError
=== RUN   TestShouldRetryWithSMS/Should_not_retry_-_other_subscription_result
=== RUN   TestShouldRetryWithSMS/Should_not_retry_-_other_subscription_error
2026-01-24T01:03:19.901Z	DEBUG	service/subscription.go:1312	No subscriptions found for invalid MSISDN, skipping cleanup	{"msisdn": "233123456789"}
=== RUN   TestShouldRetryWithSMS/Should_not_retry_-_no_response_data
2026-01-24T01:03:19.901Z	DEBUG	service/subscription.go:1312	No subscriptions found for invalid MSISDN, skipping cleanup	{"msisdn": "233123456789"}
=== RUN   TestShouldRetryWithSMS/Should_not_retry_-_empty_response_data
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-2", "duration": "24.14µs"}
--- PASS: TestShouldRetryWithSMS (0.00s)
    --- PASS: TestShouldRetryWithSMS/Should_retry_-_OPTIN_CONFIG_NOT_FOUND_in_subscriptionResult (0.00s)
    --- PASS: TestShouldRetryWithSMS/Should_retry_-_OPTIN_CONFIG_NOT_FOUND_in_subscriptionError (0.00s)
    --- PASS: TestShouldRetryWithSMS/Should_not_retry_-_other_subscription_result (0.00s)
    --- PASS: TestShouldRetryWithSMS/Should_not_retry_-_other_subscription_error (0.00s)
    --- PASS: TestShouldRetryWithSMS/Should_not_retry_-_no_response_data (0.00s)
    --- PASS: TestShouldRetryWithSMS/Should_not_retry_-_empty_response_data (0.00s)
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id", "duration": "21.262µs"}
PASS
2026-01-24T01:03:19.901Z	DEBUG	service/subscription.go:1312	No subscriptions found for invalid MSISDN, skipping cleanup	{"msisdn": "233123456789"}
2026-01-24T01:03:19.901Z	INFO	service/subscription.go:1288	Invalid MSISDN cleanup completed successfully	{"msisdn": "233123456789", "productId": 123, "requestId": "test-request-id-3", "duration": "6.043µs"}
coverage: 12.7% of statements
ok  	github.com/seidu626/subscription-manager/subscription-external/internal/service	(cached)	coverage: 12.7% of statements
	github.com/seidu626/subscription-manager/subscription-external/internal/transport		coverage: 0.0% of statements
=== RUN   TestMSISDNValidatorIntegration
2026-01-24T01:03:12.252Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestMSISDNValidatorIntegration (0.00s)
=== RUN   TestNetworkResilientClientIntegration
2026-01-24T01:03:12.254Z	WARN	utils/network_resilience.go:160	Retryable error occurred, will retry	{"attempt": 1, "error": "error when dialing 127.0.0.1:8080: dial tcp4 127.0.0.1:8080: connect: connection refused"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).doWithRetryInternal
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:160
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).DoWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:126
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).HealthCheck
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:293
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestNetworkResilientClientIntegration
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/integration_test.go:59
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:12.254Z	DEBUG	utils/network_resilience.go:144	Retrying request after delay	{"attempt": 1, "delay": "414ms"}
2026-01-24T01:03:12.668Z	WARN	utils/network_resilience.go:160	Retryable error occurred, will retry	{"attempt": 2, "error": "error when dialing 127.0.0.1:8080: dial tcp4 127.0.0.1:8080: connect: connection refused"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).doWithRetryInternal
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:160
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).doWithRetryInternal
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:163
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).DoWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:126
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).HealthCheck
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:293
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestNetworkResilientClientIntegration
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/integration_test.go:59
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:12.668Z	DEBUG	utils/network_resilience.go:144	Retrying request after delay	{"attempt": 2, "delay": "953ms"}
2026-01-24T01:03:13.622Z	INFO	utils/network_resilience.go:107	Circuit breaker state change	{"name": "NetworkResilientClient", "from": "closed", "to": "open", "timestamp": "2026-01-24T01:03:13Z"}
2026-01-24T01:03:13.622Z	WARN	utils/network_resilience.go:160	Retryable error occurred, will retry	{"attempt": 3, "error": "error when dialing 127.0.0.1:8080: dial tcp4 127.0.0.1:8080: connect: connection refused"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).doWithRetryInternal
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:160
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).doWithRetryInternal
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:163
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).doWithRetryInternal
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:163
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).DoWithRetry
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:126
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*NetworkResilientClient).HealthCheck
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/network_resilience.go:293
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestNetworkResilientClientIntegration
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/integration_test.go:59
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:13.622Z	DEBUG	utils/network_resilience.go:144	Retrying request after delay	{"attempt": 3, "delay": "2.205s"}
--- PASS: TestNetworkResilientClientIntegration (3.58s)
=== RUN   TestBatchProcessorIntegration
--- PASS: TestBatchProcessorIntegration (0.00s)
=== RUN   TestComponentIntegration
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestComponentIntegration (0.00s)
=== RUN   TestConfigurationIntegration
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestConfigurationIntegration (0.00s)
=== RUN   TestConfigurableTelcoPrefixes
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:109	Using configured telecom prefixes	{"operator_count": 3}
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:142	Updated telecom prefixes from configuration	{"operator_count": 1}
2026-01-24T01:03:15.827Z	DEBUG	utils/msisdn_validator.go:147	Operator prefixes updated	{"operator": "NewOperator", "prefixes": ["23360", "23361", "23362"]}
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:546	Updated cache expiry	{"new_expiry": "45m0s"}
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:558	Reloaded telecom prefixes from configuration	{"operator_count": 3}
2026-01-24T01:03:15.827Z	DEBUG	utils/msisdn_validator.go:563	Operator prefixes reloaded	{"operator": "CustomMTN", "prefixes": ["23324", "23325", "23326"]}
2026-01-24T01:03:15.827Z	DEBUG	utils/msisdn_validator.go:563	Operator prefixes reloaded	{"operator": "CustomAirtel", "prefixes": ["23320", "23327", "23328"]}
2026-01-24T01:03:15.827Z	DEBUG	utils/msisdn_validator.go:563	Operator prefixes reloaded	{"operator": "CustomVodafone", "prefixes": ["23323", "23333"]}
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:571	Configuration reloaded and cache cleared
--- PASS: TestConfigurableTelcoPrefixes (0.00s)
=== RUN   TestDefaultPrefixesFallback
2026-01-24T01:03:15.827Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestDefaultPrefixesFallback (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorCreation
--- PASS: TestOptimizedMSISDNGeneratorCreation (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorStats
--- PASS: TestOptimizedMSISDNGeneratorStats (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorConfiguration
--- PASS: TestOptimizedMSISDNGeneratorConfiguration (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorResetStats
--- PASS: TestOptimizedMSISDNGeneratorResetStats (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorTelcoPrefixes
--- PASS: TestOptimizedMSISDNGeneratorTelcoPrefixes (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorRandomMSISDN
--- PASS: TestOptimizedMSISDNGeneratorRandomMSISDN (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorMSISDNLengthValidation
=== RUN   TestOptimizedMSISDNGeneratorMSISDNLengthValidation/3-digit_prefix
=== RUN   TestOptimizedMSISDNGeneratorMSISDNLengthValidation/5-digit_prefix
=== RUN   TestOptimizedMSISDNGeneratorMSISDNLengthValidation/6-digit_prefix
--- PASS: TestOptimizedMSISDNGeneratorMSISDNLengthValidation (0.00s)
    --- PASS: TestOptimizedMSISDNGeneratorMSISDNLengthValidation/3-digit_prefix (0.00s)
    --- PASS: TestOptimizedMSISDNGeneratorMSISDNLengthValidation/5-digit_prefix (0.00s)
    --- PASS: TestOptimizedMSISDNGeneratorMSISDNLengthValidation/6-digit_prefix (0.00s)
=== RUN   TestOptimizedMSISDNGeneratorBloomFilterIntegration
--- PASS: TestOptimizedMSISDNGeneratorBloomFilterIntegration (0.00s)
=== RUN   TestGenerateBatchMSISDNSOptimized_DeadlockPrevention
2026-01-24T01:03:15.828Z	INFO	utils/msisdn_generator_optimized.go:210	Starting MSISDN batch generation	{"requested": 100, "workers": 5, "maxConcurrent": 5}
--- PASS: TestGenerateBatchMSISDNSOptimized_DeadlockPrevention (0.00s)
=== RUN   TestLoadTigoUserbaseData
--- PASS: TestLoadTigoUserbaseData (0.00s)
=== RUN   TestGenerateMSISDNWithPatterns
--- PASS: TestGenerateMSISDNWithPatterns (0.00s)
=== RUN   TestMSISDNDataLoader
--- PASS: TestMSISDNDataLoader (0.00s)
=== RUN   TestWeightedPrefixSelection
--- PASS: TestWeightedPrefixSelection (0.00s)
=== RUN   TestPatternBasedGeneration
--- PASS: TestPatternBasedGeneration (0.00s)
=== RUN   TestValidMSISDNFormat
--- PASS: TestValidMSISDNFormat (0.00s)
=== RUN   TestPatternDetection
--- PASS: TestPatternDetection (0.00s)
=== RUN   TestMSISDNValidatorStandalone
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestMSISDNValidatorStandalone (0.00s)
=== RUN   TestConfigurablePrefixes
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:109	Using configured telecom prefixes	{"operator_count": 3}
--- PASS: TestConfigurablePrefixes (0.00s)
=== RUN   TestRuntimePrefixUpdates
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:142	Updated telecom prefixes from configuration	{"operator_count": 1}
2026-01-24T01:03:15.834Z	DEBUG	utils/msisdn_validator.go:147	Operator prefixes updated	{"operator": "NewOperator", "prefixes": ["23360", "23361", "23362"]}
--- PASS: TestRuntimePrefixUpdates (0.00s)
=== RUN   TestConfigurationReload
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:546	Updated cache expiry	{"new_expiry": "45m0s"}
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:558	Reloaded telecom prefixes from configuration	{"operator_count": 1}
2026-01-24T01:03:15.834Z	DEBUG	utils/msisdn_validator.go:563	Operator prefixes reloaded	{"operator": "ReloadedOperator", "prefixes": ["23370", "23371"]}
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:571	Configuration reloaded and cache cleared
--- PASS: TestConfigurationReload (0.00s)
=== RUN   TestMSISDNFormatValidation
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestMSISDNFormatValidation (0.00s)
=== RUN   TestStatisticsTracking
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestStatisticsTracking (0.00s)
=== RUN   TestCacheFunctionality
2026-01-24T01:03:15.834Z	INFO	utils/msisdn_validator.go:112	Using default Ghana telecom prefixes	{"operator_count": 4}
--- PASS: TestCacheFunctionality (0.15s)
=== RUN   TestPanicHandler_RecoverPanic
2026-01-24T01:03:15.984Z	ERROR	utils/panic_handler.go:322	PANIC RECOVERED	{"panic_value": "test panic", "panic_type": "string", "caller": "/usr/lib/go/src/runtime/panic.go:783", "goroutine_info": "Goroutines: 24", "timestamp": "2026-01-24T01:03:15Z", "panic_depth": 1, "total_panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:322
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:33
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:34
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:15.984Z	ERROR	utils/panic_handler.go:334	PANIC STACK TRACE	{"stack_trace": "goroutine 16 [running]:\nruntime/debug.Stack()\n\t/usr/lib/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).getStackTrace(0xc0002e8270)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:441 +0x58\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic(0xc0002e8270, {0x871e00, 0x9dff90}, {0x0, 0x0})\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:310 +0x17f\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic(0xc0002e8270)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214 +0x68\npanic({0x871e00?, 0x9dff90?})\n\t/usr/lib/go/src/runtime/panic.go:783 +0x132\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic.func1(0xc0002ba100?)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:33 +0x4d\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic(0xc0003348c0)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:34 +0x7a\ntesting.tRunner(0xc0003348c0, 0x969550)\n\t/usr/lib/go/src/testing/testing.go:1934 +0xea\ncreated by testing.(*T).Run in goroutine 1\n\t/usr/lib/go/src/testing/testing.go:1997 +0x465\n"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:334
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:33
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:34
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:15.984Z	ERROR	utils/panic_handler.go:482	PANIC SYSTEM INFO	{"alloc_mb": 1, "total_alloc_mb": 4, "sys_mb": 13, "num_gc": 2, "goroutines": 24, "cpu_count": 20}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).logSystemInfo
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:482
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:350
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:33
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:34
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:15.984Z	INFO	utils/panic_handler.go:498	Executing panic recovery logic	{"panic_value": "test panic", "timeout": "5s"}
2026-01-24T01:03:15.984Z	INFO	utils/panic_handler.go:533	Performing recovery actions	{"panic_value": "test panic"}
2026-01-24T01:03:15.984Z	INFO	utils/panic_handler.go:538	Forcing garbage collection
2026-01-24T01:03:15.985Z	INFO	utils/panic_handler.go:544	Memory stats after recovery	{"alloc_mb": 0, "total_alloc_mb": 4, "sys_mb": 13}
2026-01-24T01:03:15.985Z	INFO	utils/panic_handler.go:522	Panic recovery completed successfully
    panic_handler_test.go:38: Panic was successfully recovered
--- PASS: TestPanicHandler_RecoverPanic (0.00s)
=== RUN   TestPanicHandler_SafeGo
2026-01-24T01:03:15.985Z	ERROR	utils/panic_handler.go:322	PANIC RECOVERED	{"panic_value": "test panic in goroutine", "panic_type": "string", "caller": "/usr/lib/go/src/runtime/panic.go:783", "goroutine_info": "Goroutines: 37", "timestamp": "2026-01-24T01:03:15Z", "panic_depth": 1, "total_panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:322
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:63
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:239
2026-01-24T01:03:15.985Z	ERROR	utils/panic_handler.go:334	PANIC STACK TRACE	{"stack_trace": "goroutine 32 [running]:\nruntime/debug.Stack()\n\t/usr/lib/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).getStackTrace(0xc000414340)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:441 +0x58\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic(0xc000414340, {0x871e00, 0x9dffb0}, {0x0, 0x0})\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:310 +0x17f\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic(0xc000414340)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214 +0x68\npanic({0x871e00?, 0x9dffb0?})\n\t/usr/lib/go/src/runtime/panic.go:783 +0x132\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_SafeGo.func1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:63 +0x51\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).SafeGo.func1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:239 +0x54\ncreated by github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).SafeGo in goroutine 19\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:237 +0x91\n"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:334
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:63
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:239
2026-01-24T01:03:15.985Z	ERROR	utils/panic_handler.go:482	PANIC SYSTEM INFO	{"alloc_mb": 0, "total_alloc_mb": 4, "sys_mb": 13, "num_gc": 3, "goroutines": 37, "cpu_count": 20}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).logSystemInfo
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:482
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:350
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).RecoverPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:214
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:63
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:239
2026-01-24T01:03:15.985Z	INFO	utils/panic_handler.go:498	Executing panic recovery logic	{"panic_value": "test panic in goroutine", "timeout": "5s"}
--- PASS: TestPanicHandler_SafeGo (0.00s)
2026-01-24T01:03:15.985Z	INFO	utils/panic_handler.go:533	Performing recovery actions	{"panic_value": "test panic in goroutine"}
2026-01-24T01:03:15.985Z	INFO	utils/panic_handler.go:538	Forcing garbage collection
=== RUN   TestPanicHandler_HandleFatalError
2026-01-24T01:03:15.986Z	ERROR	utils/panic_handler.go:553	FATAL ERROR	{"error": "test fatal error", "error_type": "*utils.testError", "timestamp": "2026-01-24T01:03:15Z", "context": {"component":"test","operation":"test-operation"}}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandleFatalError
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:553
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_HandleFatalError
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:101
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:15.986Z	ERROR	utils/panic_handler.go:482	PANIC SYSTEM INFO	{"alloc_mb": 0, "total_alloc_mb": 4, "sys_mb": 14, "num_gc": 4, "goroutines": 50, "cpu_count": 20}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).logSystemInfo
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:482
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandleFatalError
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:561
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_HandleFatalError
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:101
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:15.986Z	INFO	utils/panic_handler.go:544	Memory stats after recovery	{"alloc_mb": 0, "total_alloc_mb": 4, "sys_mb": 14}
2026-01-24T01:03:15.986Z	INFO	utils/panic_handler.go:522	Panic recovery completed successfully
2026-01-24T01:03:15.986Z	ERROR	utils/panic_handler.go:565	FATAL ERROR STACK TRACE	{"stack_trace": "goroutine 102 [running]:\nruntime/debug.Stack()\n\t/usr/lib/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandleFatalError(0xc0002e80d0, {0x9e2fa0, 0xc0002e0600}, 0xc0000be660)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:564 +0x497\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_HandleFatalError(0xc00060a380?)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:101 +0x14e\ntesting.tRunner(0xc00060a380, 0x969538)\n\t/usr/lib/go/src/testing/testing.go:1934 +0xea\ncreated by testing.(*T).Run in goroutine 1\n\t/usr/lib/go/src/testing/testing.go:1997 +0x465\n"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandleFatalError
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:565
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestPanicHandler_HandleFatalError
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler_test.go:101
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestPanicHandler_HandleFatalError (0.00s)
=== RUN   TestPanicHandler_DefaultConfig
--- PASS: TestPanicHandler_DefaultConfig (0.00s)
=== RUN   TestPanicHandler_ConfigValidation
--- PASS: TestPanicHandler_ConfigValidation (0.00s)
=== RUN   TestPanicHandler_SelfProtection
--- PASS: TestPanicHandler_SelfProtection (0.00s)
=== RUN   TestPanicHandler_MemoryManagement
--- PASS: TestPanicHandler_MemoryManagement (0.00s)
=== RUN   TestPanicHandler_Alerting
[test-console] HIGH ALERT: High panic rate detected - system under stress
  Panic Type: string
  Panic Depth: 1
  Total Panics: 1
  Memory Usage: 0 MB
  Timestamp: 2026-01-24T01:03:15Z
  Context: map[max_panics_per_second:1]

    panic_handler_test.go:235: Alert channels status not exposed in GetStatus
--- PASS: TestPanicHandler_Alerting (0.00s)
=== RUN   TestPanicHandler_PerformanceOptimization
--- PASS: TestPanicHandler_PerformanceOptimization (0.10s)
=== RUN   TestPanicHandler_RecoveryState
--- PASS: TestPanicHandler_RecoveryState (0.00s)
=== RUN   TestPanicHandler_ConfigurationValidation
--- PASS: TestPanicHandler_ConfigurationValidation (0.00s)
=== RUN   TestWorkerWrapper_WrapWorker
2026-01-24T01:03:16.100Z	DEBUG	utils/worker_wrapper.go:92	WORKER SUCCESS	{"worker_name": "test-worker", "execution_time": "10.466469ms", "total_executions": 1, "successful_executions": 1}
2026-01-24T01:03:16.110Z	ERROR	utils/worker_wrapper.go:82	WORKER FAILED	{"worker_name": "test-worker", "error": "test error", "execution_time": "10.22952ms", "total_executions": 2, "failed_executions": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorker.(*WorkerWrapper).WrapWorker.func4
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:82
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorker
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:61
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestWorkerWrapper_WrapWorker (0.02s)
=== RUN   TestWorkerWrapper_WrapWorkerWithPanic
2026-01-24T01:03:16.121Z	ERROR	utils/worker_wrapper.go:54	WORKER PANIC	{"worker_name": "panic-test-worker", "panic_value": "test panic in worker", "panic_type": "string", "execution_time": "10.304201ms", "total_executions": 1, "panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:54
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:101
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:114
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:118
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.121Z	ERROR	utils/panic_handler.go:322	PANIC RECOVERED	{"panic_value": "test panic in worker", "panic_type": "string", "caller": "/usr/lib/go/src/runtime/panic.go:783", "goroutine_info": "Goroutines: 138", "timestamp": "2026-01-24T01:03:16Z", "panic_depth": 1, "total_panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:322
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:101
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:114
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:118
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.121Z	ERROR	utils/panic_handler.go:334	PANIC STACK TRACE	{"stack_trace": "goroutine 242 [running]:\nruntime/debug.Stack()\n\t/usr/lib/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).getStackTrace(0xc0002e8820)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:441 +0x58\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic(0xc0002e8820, {0x871e00, 0x9e0140}, {0x0, 0x0})\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:310 +0x17f\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3.1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65 +0x77b\npanic({0x871e00?, 0x9e0140?})\n\t/usr/lib/go/src/runtime/panic.go:783 +0x132\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:101 +0x2b\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71 +0xc9\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func2(0xc00013c000, 0x0?)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:114 +0x4b\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic(0xc00013c000)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:118 +0x17c\ntesting.tRunner(0xc00013c000, 0x9695c0)\n\t/usr/lib/go/src/testing/testing.go:1934 +0xea\ncreated by testing.(*T).Run in goroutine 1\n\t/usr/lib/go/src/testing/testing.go:1997 +0x465\n"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:334
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:101
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:114
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:118
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.121Z	ERROR	utils/panic_handler.go:482	PANIC SYSTEM INFO	{"alloc_mb": 1, "total_alloc_mb": 4, "sys_mb": 14, "num_gc": 8, "goroutines": 138, "cpu_count": 20}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).logSystemInfo
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:482
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:350
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:101
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.(*WorkerWrapper).WrapWorker.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:114
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_WrapWorkerWithPanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:118
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.121Z	INFO	utils/panic_handler.go:498	Executing panic recovery logic	{"panic_value": "test panic in worker", "timeout": "5s"}
2026-01-24T01:03:16.121Z	INFO	utils/panic_handler.go:533	Performing recovery actions	{"panic_value": "test panic in worker"}
2026-01-24T01:03:16.121Z	INFO	utils/panic_handler.go:538	Forcing garbage collection
2026-01-24T01:03:16.122Z	INFO	utils/panic_handler.go:544	Memory stats after recovery	{"alloc_mb": 1, "total_alloc_mb": 4, "sys_mb": 14}
2026-01-24T01:03:16.122Z	INFO	utils/panic_handler.go:522	Panic recovery completed successfully
--- PASS: TestWorkerWrapper_WrapWorkerWithPanic (0.01s)
=== RUN   TestWorkerWrapper_SafeGo
2026-01-24T01:03:16.132Z	DEBUG	utils/worker_wrapper.go:92	WORKER SUCCESS	{"worker_name": "safe-go-test-worker", "execution_time": "10.171971ms", "total_executions": 1, "successful_executions": 1}
2026-01-24T01:03:16.142Z	ERROR	utils/worker_wrapper.go:54	WORKER PANIC	{"worker_name": "safe-go-test-worker", "panic_value": "test panic in SafeGo", "panic_type": "string", "execution_time": "10.236176ms", "total_executions": 2, "panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:54
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_SafeGo.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:176
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:171
2026-01-24T01:03:16.142Z	ERROR	utils/panic_handler.go:322	PANIC RECOVERED	{"panic_value": "test panic in SafeGo", "panic_type": "string", "caller": "/usr/lib/go/src/runtime/panic.go:783", "goroutine_info": "Goroutines: 151", "timestamp": "2026-01-24T01:03:16Z", "panic_depth": 1, "total_panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:322
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_SafeGo.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:176
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:171
2026-01-24T01:03:16.142Z	ERROR	utils/panic_handler.go:334	PANIC STACK TRACE	{"stack_trace": "goroutine 272 [running]:\nruntime/debug.Stack()\n\t/usr/lib/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).getStackTrace(0xc000414410)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:441 +0x58\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic(0xc000414410, {0x871e00, 0x9e0130}, {0x0, 0x0})\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:310 +0x17f\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1.1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65 +0x77b\npanic({0x871e00?, 0x9e0130?})\n\t/usr/lib/go/src/runtime/panic.go:783 +0x132\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_SafeGo.func2()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:176 +0x65\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71 +0xc9\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:171 +0x88\ncreated by github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo in goroutine 258\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:169 +0x91\n"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:334
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_SafeGo.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:176
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:171
--- PASS: TestWorkerWrapper_SafeGo (0.02s)
2026-01-24T01:03:16.142Z	ERROR	utils/panic_handler.go:482	PANIC SYSTEM INFO	{"alloc_mb": 1, "total_alloc_mb": 4, "sys_mb": 14, "num_gc": 9, "goroutines": 151, "cpu_count": 20}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).logSystemInfo
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:482
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:350
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_SafeGo.func2
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:176
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1.(*WorkerWrapper).WrapWorker.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).SafeGo.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:171
2026-01-24T01:03:16.142Z	INFO	utils/panic_handler.go:498	Executing panic recovery logic	{"panic_value": "test panic in SafeGo", "timeout": "5s"}
2026-01-24T01:03:16.142Z	INFO	utils/panic_handler.go:533	Performing recovery actions	{"panic_value": "test panic in SafeGo"}
2026-01-24T01:03:16.142Z	INFO	utils/panic_handler.go:538	Forcing garbage collection
=== RUN   TestWorkerWrapper_Metrics
2026-01-24T01:03:16.143Z	INFO	utils/panic_handler.go:544	Memory stats after recovery	{"alloc_mb": 1, "total_alloc_mb": 4, "sys_mb": 14}
2026-01-24T01:03:16.143Z	INFO	utils/panic_handler.go:522	Panic recovery completed successfully
2026-01-24T01:03:16.163Z	DEBUG	utils/worker_wrapper.go:92	WORKER SUCCESS	{"worker_name": "metrics-test-worker", "execution_time": "20.110262ms", "total_executions": 1, "successful_executions": 1}
2026-01-24T01:03:16.174Z	ERROR	utils/worker_wrapper.go:82	WORKER FAILED	{"worker_name": "metrics-test-worker", "error": "test error", "execution_time": "10.376452ms", "total_executions": 2, "failed_executions": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func6
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:82
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:243
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.189Z	ERROR	utils/worker_wrapper.go:54	WORKER PANIC	{"worker_name": "metrics-test-worker", "panic_value": "test panic", "panic_type": "string", "execution_time": "15.393598ms", "total_executions": 3, "panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:54
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:231
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func4
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:252
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:253
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.189Z	ERROR	utils/panic_handler.go:322	PANIC RECOVERED	{"panic_value": "test panic", "panic_type": "string", "caller": "/usr/lib/go/src/runtime/panic.go:783", "goroutine_info": "Goroutines: 162", "timestamp": "2026-01-24T01:03:16Z", "panic_depth": 1, "total_panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:322
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:231
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func4
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:252
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:253
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.189Z	ERROR	utils/panic_handler.go:334	PANIC STACK TRACE	{"stack_trace": "goroutine 193 [running]:\nruntime/debug.Stack()\n\t/usr/lib/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).getStackTrace(0xc0002e89c0)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:441 +0x58\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic(0xc0002e89c0, {0x871e00, 0x9dff90}, {0x0, 0x0})\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:310 +0x17f\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7.1()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65 +0x77b\npanic({0x871e00?, 0x9dff90?})\n\t/usr/lib/go/src/runtime/panic.go:783 +0x132\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func3()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:231 +0x2b\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7()\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71 +0xc9\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func4(0xc000136700?, 0x0?)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:252 +0x42\ngithub.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics(0xc00016fc00)\n\t/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:253 +0x22e\ntesting.tRunner(0xc00016fc00, 0x9695a0)\n\t/usr/lib/go/src/testing/testing.go:1934 +0xea\ncreated by testing.(*T).Run in goroutine 1\n\t/usr/lib/go/src/testing/testing.go:1997 +0x465\n"}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:334
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:231
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func4
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:252
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:253
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.189Z	ERROR	utils/panic_handler.go:482	PANIC SYSTEM INFO	{"alloc_mb": 1, "total_alloc_mb": 5, "sys_mb": 14, "num_gc": 10, "goroutines": 162, "cpu_count": 20}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).logSystemInfo
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:482
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*PanicHandler).HandlePanic
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/panic_handler.go:350
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7.1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:65
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func3
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:231
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.(*WorkerWrapper).WrapWorker.func7
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:71
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics.func4
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:252
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:253
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.189Z	INFO	utils/panic_handler.go:498	Executing panic recovery logic	{"panic_value": "test panic", "timeout": "5s"}
2026-01-24T01:03:16.189Z	INFO	utils/panic_handler.go:533	Performing recovery actions	{"panic_value": "test panic"}
2026-01-24T01:03:16.189Z	INFO	utils/panic_handler.go:538	Forcing garbage collection
2026-01-24T01:03:16.190Z	INFO	utils/panic_handler.go:544	Memory stats after recovery	{"alloc_mb": 1, "total_alloc_mb": 5, "sys_mb": 14}
2026-01-24T01:03:16.190Z	INFO	utils/panic_handler.go:522	Panic recovery completed successfully
2026-01-24T01:03:16.190Z	INFO	utils/worker_wrapper.go:246	WORKER HEALTH STATUS	{"worker_name": "metrics-test-worker", "total_executions": 3, "successful_executions": 1, "failed_executions": 2, "panic_count": 1, "success_rate_percent": 33.33333333333333, "panic_rate_percent": 33.33333333333333, "average_execution_time": "10.162238ms", "total_execution_time": "30.486714ms", "last_execution_time": "2026-01-24T01:03:16.189Z"}
2026-01-24T01:03:16.190Z	WARN	utils/worker_wrapper.go:261	Worker has high panic rate	{"worker_name": "metrics-test-worker", "panic_rate_percent": 33.33333333333333, "panic_count": 1}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).LogHealthStatus
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:261
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:292
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
2026-01-24T01:03:16.190Z	WARN	utils/worker_wrapper.go:270	Worker has low success rate	{"worker_name": "metrics-test-worker", "success_rate_percent": 33.33333333333333, "failed_executions": 2}
github.com/seidu626/subscription-manager/subscription-external/internal/utils.(*WorkerWrapper).LogHealthStatus
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper.go:270
github.com/seidu626/subscription-manager/subscription-external/internal/utils.TestWorkerWrapper_Metrics
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/utils/worker_wrapper_test.go:292
testing.tRunner
	/usr/lib/go/src/testing/testing.go:1934
--- PASS: TestWorkerWrapper_Metrics (0.05s)
=== RUN   TestWorkerWrapper_ResetMetrics
2026-01-24T01:03:16.200Z	DEBUG	utils/worker_wrapper.go:92	WORKER SUCCESS	{"worker_name": "reset-metrics-test-worker", "execution_time": "10.148851ms", "total_executions": 1, "successful_executions": 1}
2026-01-24T01:03:16.200Z	INFO	utils/worker_wrapper.go:225	Worker metrics reset	{"worker_name": "reset-metrics-test-worker"}
--- PASS: TestWorkerWrapper_ResetMetrics (0.01s)
PASS
coverage: 42.1% of statements
ok  	github.com/seidu626/subscription-manager/subscription-external/internal/utils	(cached)	coverage: 42.1% of statements
=== RUN   TestProcessorCreation
--- PASS: TestProcessorCreation (0.00s)
=== RUN   TestProcessorWithMockTracker
2026-01-24T01:03:12.250Z	INFO	worker/resubscription_processor.go:295	Configuration updated	{"batch_size": 20, "max_concurrency": 3, "checkpoint_interval": 10}
--- PASS: TestProcessorWithMockTracker (0.00s)
=== RUN   TestProcessorStatistics
2026-01-24T01:03:12.250Z	INFO	worker/resubscription_processor.go:341	Processing statistics reset
2026-01-24T01:03:12.250Z	INFO	worker/resubscription_processor.go:332	Processing results cleared
--- PASS: TestProcessorStatistics (0.00s)
=== RUN   TestProcessorExport
--- PASS: TestProcessorExport (0.00s)
=== RUN   TestProcessorPauseResume
2026-01-24T01:03:12.251Z	INFO	worker/resubscription_processor.go:232	Processing resumed
--- PASS: TestProcessorPauseResume (0.00s)
=== RUN   TestProcessorGracefulShutdown
2026-01-24T01:03:12.251Z	ERROR	worker/resubscription_processor.go:375	PANIC RECOVERED in processChargingFailures	{"panic_value": "runtime error: invalid memory address or nil pointer dereference", "panic_type": "runtime.errorString", "timestamp": "2026-01-24T01:03:12Z"}
github.com/seidu626/subscription-manager/subscription-external/internal/worker.(*ResubscriptionProcessor).processChargingFailures.func1
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/worker/resubscription_processor.go:375
runtime.gopanic
	/usr/lib/go/src/runtime/panic.go:783
runtime.panicmem
	/usr/lib/go/src/runtime/panic.go:262
runtime.sigpanic
	/usr/lib/go/src/runtime/signal_unix.go:925
github.com/seidu626/subscription-manager/subscription-external/internal/worker.(*ResubscriptionProcessor).processChargingFailures
	/home/xper626/Documents/repositories/timwe-subscription/services/subscription-external/internal/worker/resubscription_processor.go:429
2026-01-24T01:03:12.251Z	INFO	worker/resubscription_processor.go:1036	Starting graceful shutdown	{"timeout": "1s"}
2026-01-24T01:03:12.251Z	INFO	worker/resubscription_processor.go:1058	All processing completed, proceeding with cleanup
2026-01-24T01:03:12.251Z	INFO	worker/resubscription_processor.go:1078	Graceful shutdown completed	{"total_processed": 0, "successful": 0, "failed": 0, "skipped": 0}
--- PASS: TestProcessorGracefulShutdown (0.00s)
=== RUN   TestProcessorTimeRangeFilter
--- PASS: TestProcessorTimeRangeFilter (0.00s)
PASS
coverage: 8.6% of statements
ok  	github.com/seidu626/subscription-manager/subscription-external/internal/worker	(cached)	coverage: 8.6% of statements
testing: warning: no tests to run
PASS
coverage: [no statements]
ok  	github.com/seidu626/subscription-manager/subscription-external/services/subscription-external/internal/service	(cached)	coverage: [no statements] [no tests to run]
✅ Subscription External Service tests completed
🧪 Testing Subscription Service...
	github.com/seidu626/subscription-manager/subscription/cmd		coverage: 0.0% of statements
?   	github.com/seidu626/subscription-manager/subscription/internal/domain	[no test files]
	github.com/seidu626/subscription-manager/subscription/internal/handler		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription/internal/middleware		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription/internal/repository		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription/internal/service		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription/internal/transport		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/subscription/internal/utils		coverage: 0.0% of statements
✅ Subscription Service tests completed
🧪 Testing Billing Service...
	github.com/seidu626/subscription-manager/billing/cmd		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/billing/internal/config		coverage: 0.0% of statements
?   	github.com/seidu626/subscription-manager/billing/internal/domain	[no test files]
	github.com/seidu626/subscription-manager/billing/internal/handler		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/billing/internal/repository		coverage: 0.0% of statements
=== RUN   TestProcessPayment
--- PASS: TestProcessPayment (0.00s)
PASS
coverage: 17.9% of statements
ok  	github.com/seidu626/subscription-manager/billing/internal/service	(cached)	coverage: 17.9% of statements
	github.com/seidu626/subscription-manager/billing/internal/transport		coverage: 0.0% of statements
✅ Billing Service tests completed
🧪 Testing Notification Service...
	github.com/seidu626/subscription-manager/notification/cmd		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/cmd/notification-worker		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/internal/config		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/internal/dispatcher		coverage: 0.0% of statements
?   	github.com/seidu626/subscription-manager/notification/internal/domain	[no test files]
	github.com/seidu626/subscription-manager/notification/internal/handler		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/internal/middleware		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/internal/repository		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/internal/service		coverage: 0.0% of statements
	github.com/seidu626/subscription-manager/notification/internal/transport		coverage: 0.0% of statements
✅ Notification Service tests completed
🧪 All tests completed!
```


#### stderr

```text
(empty)
```


### `make lint`

- Exit code: `2`


#### stdout

```text
(empty)
```


#### stderr

```text
make: *** No rule to make target 'lint'.  Stop.
```


## What I want from you (copy/paste)

Please review the change for:
- P0 security/correctness issues
- P1 missing tests or likely regressions
Then produce a minimal fix checklist and a paste-ready `@codex` comment to implement it.
