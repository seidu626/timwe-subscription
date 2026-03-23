package handler

import (
	"encoding/json"
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
	
	if msisdn == "" {
		ctx.Error("MSISDN is required", fasthttp.StatusBadRequest)
		return
	}
	
	// Find transaction by MSISDN
	// If external transaction_id is provided, we could use it for better matching
	h.logger.Info("Processing callback",
		zap.String("msisdn", msisdn),
		zap.String("external_tx_id", externalTxID),
		zap.String("status", status),
	)
	tx, err := h.findTransactionByMSISDN(msisdn)
	if err != nil {
		h.logger.Error("Failed to find transaction", zap.String("msisdn", msisdn), zap.Error(err))
		ctx.Error("Transaction not found", fasthttp.StatusNotFound)
		return
	}
	
	// Update transaction status based on callback
	var attribution domain.Attribution
	if len(tx.AttributionData) > 0 {
		json.Unmarshal(tx.AttributionData, &attribution)
	}
	
	if status == "DELIVERED" || status == "SUCCESS" || status == "CONFIRMED" {
		// Mark as subscribed
		h.txRepo.UpdateStatus(tx.ID, domain.StatusSubscribed, nil, nil)
		
		// Enqueue postback
		h.enqueuePostback(tx, domain.PostbackEventSubscribed, &attribution)
		
		h.logger.Info("Transaction confirmed via callback",
			zap.String("transaction_id", tx.ID.String()),
			zap.String("msisdn", msisdn),
			zap.String("telco", telco),
		)
	} else if status == "FAILED" || status == "CANCELLED" {
		h.txRepo.UpdateStatus(tx.ID, domain.StatusFailed, nil, nil)
		h.enqueuePostback(tx, domain.PostbackEventFailed, &attribution)
	}
	
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.WriteString(`{"status":"ok"}`)
}

// findTransactionByMSISDN finds the most recent pending transaction for an MSISDN
func (h *CallbackHandler) findTransactionByMSISDN(msisdn string) (*domain.AcquisitionTransaction, error) {
	// Try to find by TIMWE transaction ID first if available in callback
	// For now, find most recent pending transaction for this MSISDN
	query := `
		SELECT id, correlation_id, campaign_slug, msisdn, status, next_action,
		       next_action_payload, ad_provider, click_id, attribution_data,
		       ip_address, user_agent, consent_required, consent_checked,
		       consent_version, consent_timestamp, landing_version_hash,
		       timwe_transaction_id, transaction_auth_code, timwe_status,
		       created_at, updated_at
		FROM acquisition_transactions
		WHERE msisdn = $1 AND status IN ('PENDING', 'ACTION_REQUIRED', 'CONFIRM_REQUIRED')
		ORDER BY created_at DESC
		LIMIT 1
	`
	
	return h.txRepo.ScanTransaction(query, msisdn)
}

// enqueuePostback enqueues a postback using campaign templates with legacy fallback
func (h *CallbackHandler) enqueuePostback(tx *domain.AcquisitionTransaction, event domain.PostbackEvent, attribution *domain.Attribution) {
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
	campaign, campaignErr := h.campaignRepo.GetBySlug(tx.CampaignSlug)
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
			"msisdn":         tx.MSISDN,
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
