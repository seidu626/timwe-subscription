package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// CallbackHandler handles telco callback requests
type CallbackHandler struct {
	txRepo           *repository.TransactionRepository
	campaignRepo     *repository.CampaignRepository
	postbackRepo     *repository.PostbackRepository
	providerReg      *service.ProviderRegistry
	postbackTemplate *service.PostbackTemplateService
	logger           *zap.Logger
}

// NewCallbackHandler creates a new callback handler
func NewCallbackHandler(
	txRepo *repository.TransactionRepository,
	campaignRepo *repository.CampaignRepository,
	postbackRepo *repository.PostbackRepository,
	providerReg *service.ProviderRegistry,
	logger *zap.Logger,
) *CallbackHandler {
	return &CallbackHandler{
		txRepo:           txRepo,
		campaignRepo:     campaignRepo,
		postbackRepo:     postbackRepo,
		providerReg:      providerReg,
		postbackTemplate: service.NewPostbackTemplateService(logger),
		logger:           logger,
	}
}

// HandleCallback handles POST /v1/callbacks/:telco
func (h *CallbackHandler) HandleCallback(ctx *fasthttp.RequestCtx) {
	// Extract telco from path
	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	telco := parts[len(parts)-1]
	if telco == "" {
		ctx.Error("Telco is required", fasthttp.StatusBadRequest)
		return
	}

	// Parse callback payload (format depends on telco)
	var payload map[string]interface{}
	if err := json.Unmarshal(ctx.PostBody(), &payload); err != nil {
		h.logger.Error("Failed to parse callback", zap.Error(err))
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	// Extract transaction identifier (varies by telco)
	// Common fields: msisdn, transaction_id, status
	msisdn, _ := payload["msisdn"].(string)
	externalTxID, _ := payload["transaction_id"].(string)
	status, _ := payload["status"].(string)
	tenantID, _ := payload["tenant_id"].(string)
	channelID, _ := payload["channel_id"].(string)

	if externalTxID == "" {
		ctx.Error("transaction_id is required for callback correlation", fasthttp.StatusUnprocessableEntity)
		return
	}

	// Find the transaction by provider correlation id so callbacks cannot
	// mutate the newest global transaction for the same MSISDN.
	h.logger.Info("Processing callback",
		zap.String("msisdn", msisdn),
		zap.String("external_tx_id", externalTxID),
		zap.String("status", status),
	)
	tx, err := h.findTransactionByExternalIDForTenant(externalTxID, tenantID)
	if err != nil {
		h.logger.Error("Failed to find transaction", zap.String("external_tx_id", externalTxID), zap.Error(err))
		ctx.Error("Transaction not found", fasthttp.StatusNotFound)
		return
	}
	if !callbackTenantMatches(tx, tenantID) {
		h.logger.Warn("Callback tenant mismatch",
			zap.String("transaction_id", tx.ID.String()),
			zap.String("callback_tenant_id", tenantID),
		)
		ctx.Error("Tenant mismatch", fasthttp.StatusForbidden)
		return
	}

	// Update transaction status based on callback
	var attribution domain.Attribution
	if len(tx.AttributionData) > 0 {
		json.Unmarshal(tx.AttributionData, &attribution)
	}

	if status == "DELIVERED" || status == "SUCCESS" || status == "CONFIRMED" {
		if tx.Status == domain.StatusSubscribed || tx.Status == domain.StatusCharged {
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.WriteString(`{"status":"ok","idempotent":true}`)
			return
		}
		// Mark as subscribed
		h.txRepo.UpdateStatus(tx.ID, domain.StatusSubscribed, nil, nil)

		// Enqueue postback
		h.enqueuePostback(tx, domain.PostbackEventSubscribed, &attribution, channelID)

		h.logger.Info("Transaction confirmed via callback",
			zap.String("transaction_id", tx.ID.String()),
			zap.String("msisdn", msisdn),
			zap.String("telco", telco),
		)
	} else if status == "FAILED" || status == "CANCELLED" {
		h.txRepo.UpdateStatus(tx.ID, domain.StatusFailed, nil, nil)
		h.enqueuePostback(tx, domain.PostbackEventFailed, &attribution, channelID)
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.WriteString(`{"status":"ok"}`)
}

// findTransactionByExternalID finds the transaction associated with the provider callback id.
func (h *CallbackHandler) findTransactionByExternalID(externalTxID string) (*domain.AcquisitionTransaction, error) {
	return h.findTransactionByExternalIDForTenant(externalTxID, "")
}

func (h *CallbackHandler) findTransactionByExternalIDForTenant(externalTxID, tenantID string) (*domain.AcquisitionTransaction, error) {
	tenantID = strings.TrimSpace(tenantID)
	var tx *domain.AcquisitionTransaction
	var err error
	if tenantID != "" {
		tx, err = h.txRepo.FindByTenantAndTimweTransactionID(tenantID, externalTxID)
	} else {
		tx, err = h.txRepo.FindByTimweTransactionID(externalTxID)
	}
	if err != nil {
		return nil, err
	}
	if tx.TenantID == nil || strings.TrimSpace(*tx.TenantID) == "" {
		tenantID, err := h.txRepo.GetTenantIDByID(tx.ID)
		if err == nil && strings.TrimSpace(tenantID) != "" {
			tenantID = strings.TrimSpace(tenantID)
			tx.TenantID = &tenantID
		}
	}
	return tx, nil
}

func callbackTenantMatches(tx *domain.AcquisitionTransaction, callbackTenantID string) bool {
	callbackTenantID = strings.TrimSpace(callbackTenantID)
	if callbackTenantID == "" {
		return true
	}
	if tx == nil || tx.TenantID == nil {
		return false
	}
	return strings.TrimSpace(*tx.TenantID) == callbackTenantID
}

// enqueuePostback enqueues a postback using campaign templates with legacy fallback
func (h *CallbackHandler) enqueuePostback(tx *domain.AcquisitionTransaction, event domain.PostbackEvent, attribution *domain.Attribution, callbackChannelID string) {
	if attribution == nil || attribution.Provider == "" {
		h.logger.Debug("Skipping postback: no provider")
		return
	}

	// Build postback context
	pbCtx := domain.NewPostbackContext(tx, attribution)

	// Add payout if available
	if tx.ChargePayout != nil {
		pbCtx.Payout = *tx.ChargePayout
	}

	var req *http.Request
	var err error

	// Try template-driven postback first (preferred)
	var campaign *domain.Campaign
	var campaignErr error
	if tx.TenantID != nil && strings.TrimSpace(*tx.TenantID) != "" {
		campaign, campaignErr = h.campaignRepo.GetAdminByTenantAndSlug(strings.TrimSpace(*tx.TenantID), tx.CampaignSlug)
	} else {
		campaignErr = fmt.Errorf("transaction tenant is required for postback template lookup")
	}
	if campaignErr != nil {
		h.logger.Warn("Could not load campaign for postback template lookup",
			zap.String("campaign_slug", tx.CampaignSlug),
			zap.Error(campaignErr),
		)
	}
	if campaign != nil && len(campaign.PostbackRules) > 0 {
		rules, parseErr := h.postbackTemplate.ParsePostbackRules(campaign.PostbackRules)
		if parseErr == nil && rules != nil {
			// Try exact provider match first, then wildcard "*"
			template, found := h.postbackTemplate.GetTemplateForEvent(rules, event, attribution.Provider)
			if !found {
				template, found = h.postbackTemplate.GetTemplateForEvent(rules, event, "*")
			}
			if found {
				req, err = h.postbackTemplate.BuildPostbackFromTemplate(template, pbCtx)
				if err != nil {
					h.logger.Error("Failed to build postback from template",
						zap.String("event", string(event)),
						zap.String("provider", attribution.Provider),
						zap.Error(err))
					return
				}
			}
		}
	}

	// Fallback to legacy provider-based postback if no template found
	if req == nil {
		provider, providerErr := h.providerReg.Get(attribution.Provider)
		if providerErr != nil {
			h.logger.Warn("No postback template or provider found, skipping postback",
				zap.String("provider", attribution.Provider),
				zap.String("event", string(event)),
				zap.String("campaign_slug", tx.CampaignSlug),
			)
			return
		}

		outcome := map[string]interface{}{
			"transaction_id": tx.ID.String(),
			"status":         string(tx.Status),
			"msisdn_hash":    pbCtx.MSISDNHash,
		}

		req, err = provider.BuildPostback(event, attribution, outcome)
		if err != nil {
			h.logger.Warn("Legacy provider postback failed, ensure campaign has postback_rules configured",
				zap.String("provider", attribution.Provider),
				zap.String("event", string(event)),
				zap.String("campaign_slug", tx.CampaignSlug),
				zap.Error(err),
			)
			return
		}
	}

	outbox := &domain.PostbackOutbox{
		ID:                  uuid.New(),
		TenantID:            tx.TenantID,
		ChannelID:           callbackChannelIDOrCampaign(callbackChannelID, campaign),
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

	headersJSON, _ := json.Marshal(req.Header)
	outbox.Headers = string(headersJSON)

	err = h.postbackRepo.CreateOutbox(outbox)
	if err != nil {
		h.logger.Error("Failed to enqueue postback", zap.Error(err))
	}
}

func callbackChannelIDOrCampaign(callbackChannelID string, campaign *domain.Campaign) *string {
	channelID := strings.TrimSpace(callbackChannelID)
	if channelID == "" && campaign != nil && campaign.ChannelID != nil {
		channelID = strings.TrimSpace(*campaign.ChannelID)
	}
	if channelID == "" {
		return nil
	}
	return &channelID
}
