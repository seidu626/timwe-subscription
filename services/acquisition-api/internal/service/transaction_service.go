package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

const defaultPendingTransactionTTL = 10 * time.Minute

var ErrPostbackFailureRecorded = errors.New("postback failure recorded")

// TransactionService handles acquisition transaction business logic
type TransactionService struct {
	txRepo           *repository.TransactionRepository
	campaignRepo     *repository.CampaignRepository
	postbackRepo     *repository.PostbackRepository
	providerReg      *ProviderRegistry
	postbackTemplate *PostbackTemplateService
	timweClient      TIMWEClient // Will be implemented to call TIMWE API
	logger           *zap.Logger
	pendingTxTTL     time.Duration
}

// TIMWEClient interface for TIMWE API integration
type TIMWEClient interface {
	OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error)
	Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error)
}

type TenantAwareTIMWEClient interface {
	OptInWithTenant(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string, tenant TenantSubscriptionContext) (*TIMWEResponse, error)
}

type TenantSubscriptionContext struct {
	TenantID  string
	ChannelID string
}

// TIMWEResponse represents a response from TIMWE API
type TIMWEResponse struct {
	Success             bool
	TransactionID       string
	TransactionAuthCode string
	Status              string
	RequiresConfirm     bool
	Message             string
}

// NewTransactionService creates a new transaction service
func NewTransactionService(
	txRepo *repository.TransactionRepository,
	campaignRepo *repository.CampaignRepository,
	postbackRepo *repository.PostbackRepository,
	providerReg *ProviderRegistry,
	timweClient TIMWEClient,
	logger *zap.Logger,
) *TransactionService {
	return &TransactionService{
		txRepo:           txRepo,
		campaignRepo:     campaignRepo,
		postbackRepo:     postbackRepo,
		providerReg:      providerReg,
		postbackTemplate: NewPostbackTemplateService(logger),
		timweClient:      timweClient,
		logger:           logger,
		pendingTxTTL:     defaultPendingTransactionTTL,
	}
}

// SetPendingTransactionTTL overrides pending transaction reuse TTL.
func (s *TransactionService) SetPendingTransactionTTL(ttl time.Duration) {
	if ttl <= 0 {
		s.pendingTxTTL = defaultPendingTransactionTTL
		return
	}
	s.pendingTxTTL = ttl
}

// CreateTransaction creates a new acquisition transaction
func (s *TransactionService) CreateTransaction(req *domain.CreateTransactionRequest) (*domain.CreateTransactionResponse, error) {
	// Get campaign. Runtime transaction creation must resolve through tenant
	// ownership; slug-only legacy campaign lookup is not a transaction path.
	tenantKey := ""
	if req.TenantKey != nil {
		tenantKey = strings.TrimSpace(*req.TenantKey)
	}
	if tenantKey == "" {
		return nil, fmt.Errorf("tenant_key is required for campaign lookup")
	}
	campaign, err := s.campaignRepo.GetByTenantKeyAndSlug(tenantKey, req.CampaignSlug)
	if err != nil {
		return nil, fmt.Errorf("campaign not found: %w", err)
	}
	campaignTenantID := ""
	if campaign.TenantID != nil {
		campaignTenantID = strings.TrimSpace(*campaign.TenantID)
	}
	if campaignTenantID == "" {
		return nil, fmt.Errorf("campaign tenant is required for transaction creation")
	}

	// Normalize attribution
	var attribution *domain.Attribution
	if req.Provider != nil && *req.Provider != "" {
		provider, err := s.providerReg.Get(*req.Provider)
		if err != nil {
			s.logger.Warn("Provider not found, using generic", zap.String("provider", *req.Provider), zap.Error(err))
			provider, _ = s.providerReg.Get("generic")
		}

		// Convert attribution data to map[string]string
		attrMap := make(map[string]string)
		for k, v := range req.AttributionData {
			attrMap[k] = v
		}
		if req.ClickID != nil {
			attrMap["click_id"] = *req.ClickID
		}

		attribution, err = provider.Normalize(attrMap)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize attribution: %w", err)
		}
		attribution.CampaignSlug = req.CampaignSlug
	} else {
		// Generic attribution
		attribution = &domain.Attribution{
			Provider:     "generic",
			CampaignSlug: req.CampaignSlug,
		}
		if req.ClickID != nil {
			attribution.ClickID = *req.ClickID
		}
	}

	// Check for duplicate (idempotency)
	if attribution.ClickID != "" && attribution.Provider != "" {
		var existing *domain.AcquisitionTransaction
		if campaign.TenantID != nil && strings.TrimSpace(*campaign.TenantID) != "" {
			existing, err = s.txRepo.FindByTenantClickID(strings.TrimSpace(*campaign.TenantID), attribution.Provider, attribution.ClickID)
		} else {
			existing, err = s.txRepo.FindByClickID(attribution.Provider, attribution.ClickID)
		}
		if err == nil && existing != nil {
			// Return existing transaction
			return s.buildResponse(existing), nil
		}
	}

	// Check throttles
	throttles := make(map[string]interface{})
	if len(campaign.Throttles) > 0 {
		json.Unmarshal(campaign.Throttles, &throttles)
	}

	ipAddr := ""
	if req.IPAddress != nil {
		ipAddr = *req.IPAddress
	}

	msisdnToUse := req.MSISDN
	useHEMSISDN := false
	if req.HESource != nil && *req.HESource != domain.HESourceNone && req.HEMSISDN != nil && *req.HEMSISDN != "" {
		msisdnToUse = *req.HEMSISDN
		useHEMSISDN = true
	}

	normalizedMSISDN, err := normalizeMSISDNForCountry(msisdnToUse, campaign.Country)
	if err != nil {
		return nil, fmt.Errorf("invalid msisdn format: %w", err)
	}
	msisdnToUse = normalizedMSISDN

	// Idempotency by campaign+msisdn: if user already has an active/finished transaction,
	// return it instead of creating new attempts that consume throttle budget.
	reuseCutoff := time.Now().Add(-s.pendingTxTTL)
	statusesForReuse := []domain.TransactionStatus{
		domain.StatusConfirmRequired,
		domain.StatusActionRequired,
	}
	existingByMSISDN, err := s.txRepo.FindLatestByTenantCampaignAndMSISDN(
		campaignTenantID,
		campaign.Slug,
		msisdnToUse,
		statusesForReuse,
		reuseCutoff,
	)
	if err == nil && existingByMSISDN != nil {
		s.logger.Info("Returning existing transaction for campaign+msisdn",
			zap.String("campaign_slug", campaign.Slug),
			zap.String("msisdn_prefix", msisdnToUse[:min(5, len(msisdnToUse))]),
			zap.String("status", string(existingByMSISDN.Status)),
			zap.String("transaction_id", existingByMSISDN.ID.String()),
			zap.Duration("transaction_age", time.Since(existingByMSISDN.CreatedAt)),
			zap.Duration("pending_tx_ttl", s.pendingTxTTL),
		)
		return s.buildResponse(existingByMSISDN), nil
	}
	if err != nil && err.Error() != "transaction not found" {
		return nil, fmt.Errorf("failed to check existing campaign+msisdn transaction: %w", err)
	}

	throttled, err := s.txRepo.CheckThrottleForTenant(campaignTenantID, campaign.Slug, msisdnToUse, ipAddr, throttles)
	if err != nil {
		return nil, fmt.Errorf("failed to check throttle: %w", err)
	}
	if throttled {
		return nil, fmt.Errorf("request throttled")
	}

	// Validate consent if required
	if campaign.ConsentRequired && !req.ConsentChecked {
		return nil, fmt.Errorf("consent required but not checked")
	}

	// Create transaction
	correlationID := uuid.New()
	transactionID := uuid.New()

	// Log only after throttling so rejected requests don't leak details
	if useHEMSISDN {
		s.logger.Info("Using HE identity for transaction",
			zap.String("he_source", string(*req.HESource)),
			zap.String("form_msisdn_prefix", req.MSISDN[:min(5, len(req.MSISDN))]),
			zap.String("he_msisdn_prefix", msisdnToUse[:min(5, len(msisdnToUse))]),
		)
	}

	tx := &domain.AcquisitionTransaction{
		ID:              transactionID,
		CorrelationID:   correlationID,
		TenantID:        campaign.TenantID,
		CampaignSlug:    req.CampaignSlug,
		MSISDN:          msisdnToUse, // Use HE MSISDN if available
		Status:          domain.StatusPending,
		AdProvider:      &attribution.Provider,
		ClickID:         &attribution.ClickID,
		IPAddress:       req.IPAddress,
		UserAgent:       req.UserAgent,
		ConsentRequired: campaign.ConsentRequired,
		ConsentChecked:  req.ConsentChecked,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		OfferProductID:  &campaign.OfferProductID,
		PricepointID:    campaign.PricepointID,
		PartnerRoleID:   campaign.PartnerRoleID,
		// HE tracking fields
		HESource:   req.HESource,
		HEMSISDN:   req.HEMSISDN,
		HEOperator: req.HEOperator,
	}

	if campaign.ConsentVersion != nil {
		tx.ConsentVersion = campaign.ConsentVersion
		if req.ConsentChecked {
			now := time.Now()
			tx.ConsentTimestamp = &now
		}
	}

	// Store attribution data
	attrData, _ := json.Marshal(attribution)
	tx.AttributionData = attrData

	// Call TIMWE API
	partnerRoleID := ""
	if campaign.PartnerRoleID != nil && *campaign.PartnerRoleID > 0 {
		partnerRoleID = fmt.Sprintf("%d", *campaign.PartnerRoleID)
	}
	trackingFields := map[string]string{
		"click_id": attribution.ClickID,
		"campaign": attribution.CampaignSlug,
	}
	var timweResp *TIMWEResponse
	if tenantClient, ok := s.timweClient.(TenantAwareTIMWEClient); ok && campaign.TenantID != nil && campaign.ChannelID != nil {
		timweResp, err = tenantClient.OptInWithTenant(
			msisdnToUse,
			*tx.OfferProductID,
			"WEB",
			trackingFields,
			partnerRoleID,
			TenantSubscriptionContext{TenantID: *campaign.TenantID, ChannelID: *campaign.ChannelID},
		)
	} else {
		timweResp, err = s.timweClient.OptIn(
			msisdnToUse, // Use HE MSISDN if available, otherwise form MSISDN
			*tx.OfferProductID,
			"WEB",
			trackingFields,
			partnerRoleID,
		)
	}

	if err != nil {
		tx.Status = domain.StatusFailed
		s.txRepo.Create(tx)
		return nil, fmt.Errorf("TIMWE opt-in failed: %w", err)
	}

	// Update transaction with TIMWE response
	if timweResp.TransactionID != "" {
		tx.TimweTransactionID = &timweResp.TransactionID
	}
	if timweResp.TransactionAuthCode != "" {
		tx.TransactionAuthCode = &timweResp.TransactionAuthCode
	}
	if timweResp.Status != "" {
		tx.TimweStatus = &timweResp.Status
	}

	// Determine next action based on campaign flow type and TIMWE response
	var nextAction domain.NextAction
	var payload map[string]interface{}
	isOTPFlow := campaign.FlowType == domain.FlowTypeOTP
	isMixedFlow := campaign.FlowType == domain.FlowTypeMixed
	hasHEIdentity := req.HESource != nil &&
		*req.HESource != domain.HESourceNone &&
		req.HEMSISDN != nil &&
		*req.HEMSISDN != ""

	if campaign.FlowType == domain.FlowTypeClickToSMS && campaign.ShortCode != nil && campaign.SMSKeyword != nil {
		tx.Status = domain.StatusActionRequired
		nextAction = domain.NextActionOpenSMS
		smsLink := fmt.Sprintf("sms:%s?body=%s", *campaign.ShortCode, *campaign.SMSKeyword)
		payload = map[string]interface{}{
			"sms_link":   smsLink,
			"short_code": *campaign.ShortCode,
			"keyword":    *campaign.SMSKeyword,
			"fallback_steps": []string{
				"Open your SMS app",
				fmt.Sprintf("Send '%s' to %s", *campaign.SMSKeyword, *campaign.ShortCode),
				"Wait for confirmation",
			},
		}
	} else if isOTPFlow && timweResp.Success && !hasHEIdentity {
		tx.Status = domain.StatusConfirmRequired
		nextAction = domain.NextActionOTP
		payload = map[string]interface{}{
			"transaction_id": tx.ID.String(),
			"prompt":         "Please enter the confirmation code sent to your phone",
			"message":        "OTP sent successfully. Please confirm your subscription.",
		}
	} else if isMixedFlow && timweResp.RequiresConfirm {
		// Preserve mixed-flow behavior for providers that explicitly require confirmation.
		tx.Status = domain.StatusConfirmRequired
		nextAction = domain.NextActionOTP
		payload = map[string]interface{}{
			"transaction_id": tx.ID.String(),
			"prompt":         "Please enter the confirmation code sent to your phone",
			"message":        "OTP sent successfully. Please confirm your subscription.",
		}
	} else if campaign.FlowType == domain.FlowTypeRedirect && timweResp.Success {
		redirectURL, hasRedirect := resolveCampaignRedirectURL(campaign)
		if hasRedirect {
			tx.Status = domain.StatusActionRequired
			nextAction = domain.NextActionRedirect
			payload = map[string]interface{}{
				"url":          redirectURL,
				"redirect_url": redirectURL,
				"message":      "Redirecting to complete subscription...",
			}
		} else {
			tx.Status = domain.StatusSubscribed
			nextAction = domain.NextActionShowInstructions
			payload = map[string]interface{}{
				"message": "Subscription successful!",
			}
		}
	} else if timweResp.Success {
		tx.Status = domain.StatusSubscribed
		nextAction = domain.NextActionShowInstructions
		payload = map[string]interface{}{
			"message": "Subscription successful!",
		}
		// NOTE: Conversion postback is NOT fired here. It fires on charge success
		// via HandleChargeSuccess() when subscription-external notifies us.
		// This is the Mobplus requirement: postback only on actual charge.
	} else {
		tx.Status = domain.StatusFailed
		nextAction = domain.NextActionShowInstructions
		payload = map[string]interface{}{
			"message": "Subscription failed. Please try again.",
		}
	}

	tx.NextAction = &nextAction
	payloadJSON, _ := json.Marshal(payload)
	tx.NextActionPayload = payloadJSON

	// Save transaction
	err = s.txRepo.Create(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}

	return s.buildResponse(tx), nil
}

// ConfirmTransaction confirms a transaction (OTP flow)
func (s *TransactionService) ConfirmTransaction(transactionID uuid.UUID, authCode string) (*domain.TransactionStatusResponse, error) {
	// Get transaction
	tx, err := s.txRepo.GetByID(transactionID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	if tx.Status != domain.StatusConfirmRequired {
		return nil, fmt.Errorf("transaction is not in confirm_required status")
	}

	// Call TIMWE confirm
	if tx.TimweTransactionID == nil {
		return nil, fmt.Errorf("missing TIMWE transaction data")
	}

	// Fetch campaign to get product + partner role (confirm endpoint requires these)
	campaign, err := s.campaignForTransaction(tx)
	if err != nil {
		return nil, fmt.Errorf("campaign not found: %w", err)
	}
	productID := campaign.OfferProductID
	if tx.OfferProductID != nil && *tx.OfferProductID > 0 {
		productID = *tx.OfferProductID
		if campaign.OfferProductID != *tx.OfferProductID {
			s.logger.Warn("Confirm product differs from current campaign config; using transaction-scoped product",
				zap.String("transaction_id", transactionID.String()),
				zap.String("campaign_slug", tx.CampaignSlug),
				zap.Int("transaction_offer_product_id", *tx.OfferProductID),
				zap.Int("campaign_offer_product_id", campaign.OfferProductID),
			)
		}
	}

	var roleID int
	switch {
	case tx.PartnerRoleID != nil && *tx.PartnerRoleID > 0:
		roleID = *tx.PartnerRoleID
	case campaign.PartnerRoleID != nil && *campaign.PartnerRoleID > 0:
		roleID = *campaign.PartnerRoleID
	}

	partnerRoleID := ""
	if roleID > 0 {
		partnerRoleID = fmt.Sprintf("%d", roleID)
	}

	s.logger.Info("Confirming transaction with resolved TIMWE context",
		zap.String("transaction_id", transactionID.String()),
		zap.String("campaign_slug", tx.CampaignSlug),
		zap.Int("product_id", productID),
		zap.Int("partner_role_id", roleID),
	)

	timweResp, err := s.timweClient.Confirm(tx.MSISDN, productID, "WEB", partnerRoleID, authCode)
	if err != nil {
		s.logger.Warn("TIMWE confirm failed",
			zap.String("transaction_id", transactionID.String()),
			zap.String("campaign_slug", tx.CampaignSlug),
			zap.Int("product_id", productID),
			zap.String("partner_role_id", partnerRoleID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("TIMWE confirm failed: %w", err)
	}

	if !timweResp.Success {
		if isPendingConfirmStatus(timweResp.Status) {
			if timweResp.Status != "" {
				s.txRepo.UpdateTIMWEData(transactionID, *tx.TimweTransactionID, authCode, timweResp.Status)
			}
			message := normalizeProviderMessage(timweResp.Message)
			if message == "" {
				message = "Confirmation not finalized yet. Please retry."
			}
			return &domain.TransactionStatusResponse{
				TransactionID: transactionID,
				Status:        domain.StatusConfirmRequired,
				NextAction:    nil,
				Payload:       map[string]interface{}{"message": message},
			}, nil
		}

		// Update status to failed
		s.txRepo.UpdateStatus(transactionID, domain.StatusFailed, nil, nil)
		return &domain.TransactionStatusResponse{
			TransactionID: transactionID,
			Status:        domain.StatusFailed,
		}, nil
	}

	// Update to subscribed
	s.txRepo.UpdateStatus(transactionID, domain.StatusSubscribed, nil, nil)
	if timweResp.Status != "" {
		s.txRepo.UpdateTIMWEData(transactionID, *tx.TimweTransactionID, authCode, timweResp.Status)
	}

	// NOTE: Conversion postback is NOT fired here. It fires on charge success
	// via HandleChargeSuccess() when subscription-external notifies us.

	return &domain.TransactionStatusResponse{
		TransactionID: transactionID,
		Status:        domain.StatusSubscribed,
		NextAction:    nil,
		Payload:       map[string]interface{}{"message": "Subscription confirmed successfully"},
	}, nil
}

// GetTransactionStatus retrieves the current status of a transaction
func (s *TransactionService) GetTransactionStatus(transactionID uuid.UUID) (*domain.TransactionStatusResponse, error) {
	tx, err := s.txRepo.GetByID(transactionID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	var payload map[string]interface{}
	if len(tx.NextActionPayload) > 0 {
		json.Unmarshal(tx.NextActionPayload, &payload)
	}

	return &domain.TransactionStatusResponse{
		TransactionID: tx.ID,
		Status:        tx.Status,
		NextAction:    tx.NextAction,
		Payload:       payload,
	}, nil
}

// buildResponse builds a CreateTransactionResponse from a transaction
func (s *TransactionService) buildResponse(tx *domain.AcquisitionTransaction) *domain.CreateTransactionResponse {
	var payload map[string]interface{}
	if len(tx.NextActionPayload) > 0 {
		json.Unmarshal(tx.NextActionPayload, &payload)
	}

	var nextAction domain.NextAction
	if tx.NextAction != nil {
		nextAction = *tx.NextAction
	}

	return &domain.CreateTransactionResponse{
		TransactionID: tx.ID,
		CorrelationID: tx.CorrelationID,
		Status:        tx.Status,
		NextAction:    nextAction,
		Payload:       payload,
	}
}

// enqueuePostback enqueues a postback for async delivery using campaign templates
func (s *TransactionService) enqueuePostback(tx *domain.AcquisitionTransaction, event domain.PostbackEvent, attribution *domain.Attribution, campaign *domain.Campaign) error {
	if attribution == nil || attribution.Provider == "" {
		s.logger.Debug("Skipping postback: no provider")
		provider := "unknown"
		if attribution != nil {
			provider = attribution.Provider
		}
		return s.recordFailedPostback(tx, event, provider, campaign, "no provider in attribution data")
	}

	// Build postback context
	ctx := domain.NewPostbackContext(tx, attribution)

	// Add payout if available
	if tx.ChargePayout != nil {
		ctx.Payout = *tx.ChargePayout
	}

	var req *http.Request
	var err error

	// Try template-driven postback first (preferred)
	if campaign != nil && len(campaign.PostbackRules) > 0 {
		rules, parseErr := s.postbackTemplate.ParsePostbackRules(campaign.PostbackRules)
		if parseErr == nil && rules != nil {
			// Try exact provider match first, then wildcard "*"
			template, found := s.postbackTemplate.GetTemplateForEvent(rules, event, attribution.Provider)
			if !found {
				template, found = s.postbackTemplate.GetTemplateForEvent(rules, event, "*")
			}
			if found {
				req, err = s.postbackTemplate.BuildPostbackFromTemplate(template, ctx)
				if err != nil {
					s.logger.Error("Failed to build postback from template",
						zap.String("event", string(event)),
						zap.String("provider", attribution.Provider),
						zap.Error(err))
					return s.recordFailedPostback(tx, event, attribution.Provider, campaign, fmt.Sprintf("failed to build postback from template: %v", err))
				}
			}
		}
	}

	// Fallback to legacy provider-based postback if no template found
	if req == nil {
		provider, providerErr := s.providerReg.Get(attribution.Provider)
		if providerErr != nil {
			s.logger.Warn("No postback template or provider found, skipping postback",
				zap.String("provider", attribution.Provider),
				zap.String("event", string(event)),
				zap.String("campaign_slug", tx.CampaignSlug),
			)
			return s.recordFailedPostback(tx, event, attribution.Provider, campaign, fmt.Sprintf("no postback_rules configured for campaign %q and no registered provider %q", tx.CampaignSlug, attribution.Provider))
		}

		outcome := map[string]interface{}{
			"transaction_id": tx.ID.String(),
			"status":         string(tx.Status),
			"msisdn_hash":    ctx.MSISDNHash,
		}

		req, err = provider.BuildPostback(event, attribution, outcome)
		if err != nil {
			s.logger.Warn("Legacy provider postback failed, ensure campaign has postback_rules configured",
				zap.String("provider", attribution.Provider),
				zap.String("event", string(event)),
				zap.String("campaign_slug", tx.CampaignSlug),
				zap.Error(err),
			)
			return s.recordFailedPostback(tx, event, attribution.Provider, campaign, fmt.Sprintf("failed to build postback for provider %q: %v", attribution.Provider, err))
		}
	}

	// Create outbox entry
	outbox := &domain.PostbackOutbox{
		ID:                  uuid.New(),
		TenantID:            tx.TenantID,
		ChannelID:           campaignChannelID(campaign),
		TransactionID:       tx.ID,
		Event:               event,
		Provider:            attribution.Provider,
		URLTemplateRendered: req.URL.String(),
		HTTPMethod:          req.Method,
		AttemptCount:        0,
		MaxAttempts:         5,
		Status:              domain.PostbackStatusPending,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	// Serialize headers
	headersJSON, _ := json.Marshal(req.Header)
	outbox.Headers = string(headersJSON)

	err = s.postbackRepo.CreateOutbox(outbox)
	if err != nil {
		s.logger.Error("Failed to enqueue postback", zap.Error(err))
		return fmt.Errorf("failed to save postback to outbox: %w", err)
	}

	s.logger.Info("Postback enqueued",
		zap.String("transaction_id", tx.ID.String()),
		zap.String("event", string(event)),
		zap.String("provider", attribution.Provider),
		zap.String("url", req.URL.String()),
	)
	return nil
}

func (s *TransactionService) recordFailedPostback(tx *domain.AcquisitionTransaction, event domain.PostbackEvent, provider string, campaign *domain.Campaign, reason string) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "unknown"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "postback could not be built"
	}
	now := time.Now()
	outbox := &domain.PostbackOutbox{
		ID:                  uuid.New(),
		TenantID:            tx.TenantID,
		ChannelID:           campaignChannelID(campaign),
		TransactionID:       tx.ID,
		Event:               event,
		Provider:            provider,
		URLTemplateRendered: "skipped://postback",
		HTTPMethod:          "GET",
		Headers:             "{}",
		FailureReason:       &reason,
		AttemptCount:        0,
		MaxAttempts:         0,
		Status:              domain.PostbackStatusFailed,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.postbackRepo.CreateOutbox(outbox); err != nil {
		return fmt.Errorf("failed to record postback failure: %w", err)
	}
	return fmt.Errorf("%w: %s", ErrPostbackFailureRecorded, reason)
}

func campaignChannelID(campaign *domain.Campaign) *string {
	if campaign == nil || campaign.ChannelID == nil || strings.TrimSpace(*campaign.ChannelID) == "" {
		return nil
	}
	channelID := strings.TrimSpace(*campaign.ChannelID)
	return &channelID
}

// ChargeSuccessRequest represents the request from subscription-external on charge success
type ChargeSuccessRequest struct {
	TimweTransactionID string `json:"timwe_transaction_id"`
	TenantID           string `json:"tenant_id,omitempty"`
	ChannelID          string `json:"channel_id,omitempty"`
	MSISDN             string `json:"msisdn,omitempty"`
	ProductID          int    `json:"product_id,omitempty"`
	ChargedAt          string `json:"charged_at,omitempty"`
	Payout             string `json:"payout,omitempty"`
}

// HandleChargeSuccess processes a charge success notification and enqueues conversion postback
func (s *TransactionService) HandleChargeSuccess(req *ChargeSuccessRequest) error {
	if req.TimweTransactionID == "" {
		return fmt.Errorf("timwe_transaction_id is required")
	}

	tenantID := strings.TrimSpace(req.TenantID)
	var tx *domain.AcquisitionTransaction
	var err error
	if tenantID != "" {
		tx, err = s.txRepo.FindByTenantAndTimweTransactionID(tenantID, req.TimweTransactionID)
	} else {
		tx, err = s.txRepo.FindByTimweTransactionID(req.TimweTransactionID)
	}
	if err != nil {
		// Fallback: try by MSISDN if provided, searching across statuses that
		// may exist due to the confirm bug (transactions stuck in CONFIRM_REQUIRED
		// or ACTION_REQUIRED even though TIMWE processed the subscription).
		if req.MSISDN != "" && tenantID != "" {
			tx, err = s.txRepo.FindByTenantMSISDNAndStatuses(tenantID, req.MSISDN, []domain.TransactionStatus{
				domain.StatusSubscribed,
				domain.StatusConfirmRequired,
				domain.StatusActionRequired,
			})
			if err != nil {
				return fmt.Errorf("transaction not found for timwe_transaction_id=%s: %w", req.TimweTransactionID, err)
			}
		} else {
			return fmt.Errorf("transaction not found for timwe_transaction_id=%s: %w", req.TimweTransactionID, err)
		}
	}
	if err := s.verifyChargeSuccessTenant(tx, req); err != nil {
		return err
	}

	// If the transaction is in a pre-SUBSCRIBED state, advance it to SUBSCRIBED first.
	// A charge success proves the subscription succeeded on TIMWE's side.
	if tx.Status == domain.StatusConfirmRequired || tx.Status == domain.StatusActionRequired {
		s.logger.Info("Advancing transaction from pre-subscribed state on charge success",
			zap.String("transaction_id", tx.ID.String()),
			zap.String("previous_status", string(tx.Status)),
		)
		if err := s.txRepo.UpdateStatus(tx.ID, domain.StatusSubscribed, nil, nil); err != nil {
			return fmt.Errorf("failed to advance transaction to SUBSCRIBED: %w", err)
		}
		tx.Status = domain.StatusSubscribed
	}

	// Check if already processed (idempotency)
	if tx.ConversionPostbackSent {
		s.logger.Info("Conversion postback already sent, skipping",
			zap.String("transaction_id", tx.ID.String()),
			zap.String("timwe_transaction_id", req.TimweTransactionID),
		)
		return nil
	}

	// Update transaction to CHARGED status
	now := time.Now()
	tx.Status = domain.StatusCharged
	tx.ChargedAt = &now
	if req.Payout != "" {
		tx.ChargePayout = &req.Payout
	}

	// Mark postback as pending to be sent
	if err := s.txRepo.MarkCharged(tx.ID, &now, req.Payout); err != nil {
		return fmt.Errorf("failed to mark transaction as charged: %w", err)
	}

	// Get campaign for postback rules
	campaign, err := s.campaignForTransaction(tx)
	if err != nil {
		s.logger.Warn("Campaign not found for postback rules",
			zap.String("campaign_slug", tx.CampaignSlug),
			zap.Error(err))
		// Continue anyway, will use fallback postback logic
	}

	// Parse attribution data
	var attribution domain.Attribution
	if len(tx.AttributionData) > 0 {
		if err := json.Unmarshal(tx.AttributionData, &attribution); err != nil {
			s.logger.Warn("Failed to parse attribution data", zap.Error(err))
		}
	}
	if attribution.Provider == "" && tx.AdProvider != nil {
		attribution.Provider = *tx.AdProvider
	}
	if attribution.ClickID == "" && tx.ClickID != nil {
		attribution.ClickID = *tx.ClickID
	}

	// Enqueue conversion postback (Mobplus requirement: fire on charge success)
	if err := s.enqueuePostback(tx, domain.PostbackEventConversion, &attribution, campaign); err != nil {
		s.logger.Warn("Failed to enqueue conversion postback", zap.String("transaction_id", tx.ID.String()), zap.Error(err))
		if !errors.Is(err, ErrPostbackFailureRecorded) {
			return nil
		}
	}

	// Mark conversion postback as handled once a deliverable or failed outbox row exists.
	if err := s.txRepo.MarkConversionPostbackSent(tx.ID); err != nil {
		s.logger.Error("Failed to mark conversion postback sent", zap.Error(err))
		// Don't return error - postback is already enqueued
	}

	s.logger.Info("Charge success processed, conversion postback enqueued",
		zap.String("transaction_id", tx.ID.String()),
		zap.String("timwe_transaction_id", req.TimweTransactionID),
		zap.String("provider", attribution.Provider),
		zap.String("click_id", attribution.ClickID),
	)

	return nil
}

func (s *TransactionService) verifyChargeSuccessTenant(tx *domain.AcquisitionTransaction, req *ChargeSuccessRequest) error {
	if tx == nil || req == nil {
		return fmt.Errorf("transaction and charge success request are required")
	}
	requestTenantID := strings.TrimSpace(req.TenantID)
	if requestTenantID == "" {
		return nil
	}
	transactionTenantID := ""
	if tx.TenantID != nil {
		transactionTenantID = strings.TrimSpace(*tx.TenantID)
	}
	if transactionTenantID == "" {
		resolved, err := s.txRepo.GetTenantIDByID(tx.ID)
		if err != nil {
			return fmt.Errorf("failed to verify transaction tenant: %w", err)
		}
		transactionTenantID = strings.TrimSpace(resolved)
		if transactionTenantID != "" {
			tx.TenantID = &transactionTenantID
		}
	}
	if transactionTenantID == "" || transactionTenantID != requestTenantID {
		return fmt.Errorf("charge success tenant mismatch")
	}
	return nil
}

func (s *TransactionService) campaignForTransaction(tx *domain.AcquisitionTransaction) (*domain.Campaign, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	tenantID := ""
	if tx.TenantID != nil {
		tenantID = strings.TrimSpace(*tx.TenantID)
	}
	if tenantID == "" {
		resolvedTenantID, err := s.txRepo.GetTenantIDByID(tx.ID)
		if err == nil {
			tenantID = strings.TrimSpace(resolvedTenantID)
			if tenantID != "" {
				tx.TenantID = &tenantID
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to resolve transaction tenant: %w", err)
		}
	}
	if tenantID != "" {
		return s.campaignRepo.GetAdminByTenantAndSlug(tenantID, tx.CampaignSlug)
	}
	return nil, fmt.Errorf("transaction tenant is required for campaign lookup")
}

// GetTransactionByTimweID retrieves a transaction by TIMWE transaction ID
func (s *TransactionService) GetTransactionByTimweID(timweTransactionID string) (*domain.AcquisitionTransaction, error) {
	return s.txRepo.FindByTimweTransactionID(timweTransactionID)
}

// TriggerPostbackResult represents the outcome of a single provider postback enqueue.
type TriggerPostbackResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// TriggerPostback manually enqueues postbacks for a transaction. If providerOverride
// is set, only that provider is tried. Otherwise, all providers configured in the
// campaign's postback_rules for the given event are tried.
func (s *TransactionService) TriggerPostback(transactionID uuid.UUID, event domain.PostbackEvent, providerOverride string) ([]TriggerPostbackResult, error) {
	tx, err := s.txRepo.GetByID(transactionID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// Parse attribution data
	var attribution domain.Attribution
	if len(tx.AttributionData) > 0 {
		if err := json.Unmarshal(tx.AttributionData, &attribution); err != nil {
			return nil, fmt.Errorf("failed to parse attribution data: %w", err)
		}
	}

	// Get campaign for postback rules
	campaign, err := s.campaignForTransaction(tx)
	if err != nil {
		s.logger.Warn("Campaign not found for manual postback",
			zap.String("campaign_slug", tx.CampaignSlug),
			zap.Error(err))
	}

	// Determine which providers to fire postbacks for
	providers := []string{}
	if providerOverride != "" {
		providers = append(providers, providerOverride)
	} else if campaign != nil && len(campaign.PostbackRules) > 0 {
		// Try all providers configured for this event in the campaign rules
		rules, parseErr := s.postbackTemplate.ParsePostbackRules(campaign.PostbackRules)
		if parseErr == nil && rules != nil {
			if eventRules, exists := rules[string(event)]; exists {
				for provider := range eventRules {
					providers = append(providers, provider)
				}
			}
		}
	}

	// If no providers found from rules, fall back to the transaction's ad_provider
	if len(providers) == 0 {
		if attribution.Provider != "" {
			providers = append(providers, attribution.Provider)
		} else {
			return nil, fmt.Errorf("no providers found: campaign %q has no postback_rules for event %q and transaction has no ad_provider", tx.CampaignSlug, event)
		}
	}

	// Enqueue a postback for each provider
	var results []TriggerPostbackResult
	for _, provider := range providers {
		attrCopy := attribution
		attrCopy.Provider = provider
		if err := s.enqueuePostback(tx, event, &attrCopy, campaign); err != nil {
			results = append(results, TriggerPostbackResult{
				Provider: provider,
				Status:   "failed",
				Error:    err.Error(),
			})
		} else {
			results = append(results, TriggerPostbackResult{
				Provider: provider,
				Status:   "enqueued",
			})
		}
	}

	// If all failed, return error
	allFailed := true
	for _, r := range results {
		if r.Status == "enqueued" {
			allFailed = false
			break
		}
	}
	if allFailed {
		return results, fmt.Errorf("all postback providers failed for transaction %s", transactionID)
	}

	s.logger.Info("Manual postback triggered",
		zap.String("transaction_id", transactionID.String()),
		zap.String("event", string(event)),
		zap.Int("providers_attempted", len(providers)),
	)

	return results, nil
}

func resolveCampaignRedirectURL(campaign *domain.Campaign) (string, bool) {
	if campaign == nil {
		return "", false
	}

	if len(campaign.TrackingConfig) > 0 {
		var tracking map[string]interface{}
		if err := json.Unmarshal(campaign.TrackingConfig, &tracking); err == nil {
			if redirectURL, ok := extractRedirectURLFromTracking(tracking); ok {
				return redirectURL, true
			}
		}
	}

	if len(campaign.LandingPageURLs) > 0 {
		for _, candidate := range campaign.LandingPageURLs {
			if parsed, err := url.Parse(candidate); err == nil && parsed.Host != "" &&
				(parsed.Scheme == "https" || parsed.Scheme == "http") {
				return parsed.String(), true
			}
		}
	}

	return "", false
}

func extractRedirectURLFromTracking(tracking map[string]interface{}) (string, bool) {
	if raw, ok := tracking["redirect_url"].(string); ok {
		if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" &&
			(parsed.Scheme == "https" || parsed.Scheme == "http") {
			return parsed.String(), true
		}
	}

	if redirectObj, ok := tracking["redirect"].(map[string]interface{}); ok {
		if raw, ok := redirectObj["url"].(string); ok {
			if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" &&
				(parsed.Scheme == "https" || parsed.Scheme == "http") {
				return parsed.String(), true
			}
		}
	}

	return "", false
}

func isPendingConfirmStatus(status string) bool {
	switch status {
	case "OPTIN_WAITING", "WAITING_FOR_CONFIRMATION", "OPTIN_PIN_WAITING", "OPTIN_PREACTIVE_WAIT_CONF", "SUCCESS":
		return true
	default:
		return false
	}
}

func normalizeProviderMessage(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "null") || strings.EqualFold(trimmed, "nil") {
		return ""
	}
	return trimmed
}
