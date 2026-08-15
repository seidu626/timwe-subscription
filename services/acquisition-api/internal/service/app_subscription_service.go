package service

import (
	"strings"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

// AppSubscriptionService thin-wraps TransactionService for the Dayline app's
// /v1/app/subscriptions endpoints. It never forks the transaction state
// machine: create/confirm/status delegate straight to TransactionService,
// exactly as the landing page does, with msisdn/tenant sourced from the
// caller's JWT instead of the request body.
type AppSubscriptionService struct {
	txService    *TransactionService
	txRepo       *repository.TransactionRepository
	campaignRepo *repository.CampaignRepository
	optoutClient *AppOptoutClient
	logger       *zap.Logger
}

// NewAppSubscriptionService creates a new AppSubscriptionService. optoutClient
// may be nil, in which case Cancel fails closed with PROVIDER_ERROR.
func NewAppSubscriptionService(
	txService *TransactionService,
	txRepo *repository.TransactionRepository,
	campaignRepo *repository.CampaignRepository,
	optoutClient *AppOptoutClient,
	logger *zap.Logger,
) *AppSubscriptionService {
	return &AppSubscriptionService{
		txService:    txService,
		txRepo:       txRepo,
		campaignRepo: campaignRepo,
		optoutClient: optoutClient,
		logger:       logger,
	}
}

// Create wraps TransactionService.CreateTransaction. ConsentChecked is set
// unconditionally: the app contract carries no per-request consent field, and
// account-level app onboarding/ToS acceptance is treated as consent -
// see the result capsule for this assumption.
func (s *AppSubscriptionService) Create(msisdn, tenantKey, campaignSlug string) (*domain.CreateTransactionResponse, error) {
	campaignSlug = strings.TrimSpace(campaignSlug)
	if campaignSlug == "" {
		return nil, domain.NewAppError(domain.AppErrValidation, "campaign_slug is required")
	}
	tk := tenantKey
	resp, err := s.txService.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		TenantKey:      &tk,
		MSISDN:         msisdn,
		ConsentChecked: true,
	})
	if err != nil {
		return nil, mapTransactionServiceError(err)
	}
	return resp, nil
}

// Confirm wraps TransactionService.ConfirmTransaction after verifying the
// transaction belongs to the caller's msisdn. The tenant is resolved from
// the transaction row itself, not the caller's JWT: in the marketplace a
// user subscribes across tenants under one msisdn identity.
func (s *AppSubscriptionService) Confirm(ref, msisdn, pin string) (*domain.TransactionStatusResponse, error) {
	transactionID, err := uuid.Parse(strings.TrimSpace(ref))
	if err != nil {
		return nil, domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	}
	if _, _, err := s.authorize(transactionID, msisdn); err != nil {
		return nil, err
	}
	resp, err := s.txService.ConfirmTransaction(transactionID, pin)
	if err != nil {
		return nil, mapTransactionServiceError(err)
	}
	return resp, nil
}

// List returns the caller's subscriptions across every tenant.
func (s *AppSubscriptionService) List(msisdn string) ([]*domain.AppSubscription, error) {
	subs, err := s.txRepo.ListAppSubscriptionsByMSISDN(msisdn)
	if err != nil {
		return nil, err
	}
	return subs, nil
}

// Cancel triggers the existing opt-out path in subscription-external via a
// direct in-cluster gateway-trust call, after verifying the transaction
// belongs to the caller's msisdn. The opt-out is routed through the
// transaction's own tenant, which may differ from the login tenant.
func (s *AppSubscriptionService) Cancel(ref, msisdn string) error {
	transactionID, err := uuid.Parse(strings.TrimSpace(ref))
	if err != nil {
		return domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	}
	tx, tenantID, err := s.authorize(transactionID, msisdn)
	if err != nil {
		return err
	}
	tenantKey, err := s.txRepo.TenantKeyByID(tenantID)
	if err != nil || strings.TrimSpace(tenantKey) == "" {
		return domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	}

	campaign, err := s.campaignRepo.GetByTenantKeyAndSlug(tenantKey, tx.CampaignSlug)
	if err != nil {
		return domain.NewAppError(domain.AppErrNotFound, "campaign not found")
	}
	if campaign.ChannelID == nil {
		return domain.NewAppError(domain.AppErrProviderError, "campaign has no channel configured")
	}
	channel, err := s.campaignRepo.GetTenantChannelByID(tenantID, *campaign.ChannelID)
	if err != nil {
		s.logger.Error("failed to resolve channel for app subscription cancel", zap.Error(err))
		return domain.NewAppError(domain.AppErrProviderError, "failed to resolve channel")
	}

	if s.optoutClient == nil {
		return domain.NewAppError(domain.AppErrProviderError, "opt-out is not available")
	}
	if err := s.optoutClient.Optout(tenantKey, channel.ChannelKey, msisdn, campaign.OfferProductID); err != nil {
		s.logger.Error("app subscription optout failed", zap.Error(err))
		return domain.NewAppError(domain.AppErrProviderError, "failed to cancel subscription")
	}
	if err := s.txRepo.UpdateStatus(transactionID, domain.StatusCancelled, nil, nil); err != nil {
		// The provider opt-out already succeeded; surface success to the app
		// and let the provider's async notification reconcile the row.
		s.logger.Error("failed to mark transaction cancelled after optout", zap.Error(err))
	}
	return nil
}

// authorize loads the transaction and verifies it belongs to the caller's
// msisdn, returning the transaction and its owning tenant's UUID. Ownership
// is keyed on msisdn alone; the tenant comes from the transaction row so
// cross-tenant marketplace subscriptions stay manageable after login.
func (s *AppSubscriptionService) authorize(transactionID uuid.UUID, msisdn string) (*domain.AcquisitionTransaction, string, error) {
	tenantID, err := s.txRepo.GetTenantIDByID(transactionID)
	if err != nil || strings.TrimSpace(tenantID) == "" {
		return nil, "", domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	}
	tx, err := s.txRepo.GetByIDForTenant(transactionID, tenantID)
	if err != nil {
		return nil, "", domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	}
	if tx.MSISDN != msisdn {
		return nil, "", domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	}
	return tx, tenantID, nil
}

// mapTransactionServiceError translates TransactionService's plain-string
// errors into the app contract's error codes.
func mapTransactionServiceError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "request throttled"):
		return domain.NewAppError(domain.AppErrRateLimited, "too many requests, try again later")
	case strings.Contains(msg, "campaign not found"):
		return domain.NewAppError(domain.AppErrNotFound, "campaign not found")
	case strings.Contains(msg, "transaction not found"):
		return domain.NewAppError(domain.AppErrNotFound, "subscription not found")
	case strings.Contains(msg, "not in confirm_required status"):
		return domain.NewAppError(domain.AppErrConflict, "subscription is not awaiting confirmation")
	case strings.Contains(msg, "consent required"):
		return domain.NewAppError(domain.AppErrValidation, "consent required")
	default:
		return domain.NewAppError(domain.AppErrValidation, err.Error())
	}
}
