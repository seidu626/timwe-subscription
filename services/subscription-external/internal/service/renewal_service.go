package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

// RenewalService implements the opt-out/opt-in renewal strategy
type RenewalService struct {
	subscriptionService *SubscriptionService
	repo                repository.SubscriptionRepositoryInterface
	productRepo         *repository.ProductRepository
	logger              *zap.Logger
	config              *domain.RenewalConfig
}

// NewRenewalService creates a new renewal service
func NewRenewalService(
	subscriptionService *SubscriptionService,
	repo repository.SubscriptionRepositoryInterface,
	productRepo *repository.ProductRepository,
	logger *zap.Logger,
	config *domain.RenewalConfig,
) *RenewalService {
	return &RenewalService{
		subscriptionService: subscriptionService,
		repo:                repo,
		productRepo:         productRepo,
		logger:              logger,
		config:              config,
	}
}

// SendRenewalRequest performs the opt-out/opt-in renewal cycle
func (r *RenewalService) SendRenewalRequest(ctx context.Context, msisdn string, product *domain.Product, channel string) (*domain.RenewalResponse, error) {
	r.logger.Info("Starting opt-out/opt-in renewal cycle",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	// Check if opt-out/opt-in strategy is enabled
	if r.config.Strategy != domain.StrategyOptOutOptIn {
		return nil, fmt.Errorf("opt-out/opt-in strategy not enabled")
	}

	// Create renewal cycle record
	cycle := &domain.RenewalCycle{
		MSISDN:    msisdn,
		ProductID: product.ProductId,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Step 1: Opt-Out (Unsubscribe)
	optOutErr := r.OptOutForRenewal(ctx, msisdn, product, cycle)
	if optOutErr != nil {
		r.logger.Error("Opt-out failed during renewal",
			zap.String("msisdn", msisdn),
			zap.Error(optOutErr))
		cycle.OptOutStatus = "FAILED"
		r.SaveRenewalCycle(ctx, cycle)
		return &domain.RenewalResponse{
			MSISDN:       msisdn,
			ProductID:    product.ProductId,
			Success:      false,
			Status:       "OPT_OUT_FAILED",
			OptOutStatus: "FAILED",
			Error:        optOutErr.Error(),
		}, fmt.Errorf("opt-out failed: %w", optOutErr)
	}

	cycle.OptOutStatus = "SUCCESS"
	optOutTime := time.Now()
	cycle.OptOutTime = &optOutTime

	// Step 2: Wait for TIMWE to process the unsubscription
	waitTime := time.Duration(r.config.OptOutOptIn.WaitBetweenMs) * time.Millisecond
	r.logger.Info("Waiting before opt-in",
		zap.Duration("wait", waitTime))
	time.Sleep(waitTime)

	// Step 3: Opt-In (Resubscribe) - This triggers TIMWE's billing
	optInErr := r.OptInForRenewal(ctx, msisdn, product, channel, cycle)
	if optInErr != nil {
		r.logger.Error("Opt-in failed during renewal",
			zap.String("msisdn", msisdn),
			zap.Error(optInErr))
		cycle.OptInStatus = "FAILED"
		r.SaveRenewalCycle(ctx, cycle)

		// Critical: User is now unsubscribed, need to handle this
		r.HandleFailedOptIn(ctx, msisdn, product, cycle)
		return &domain.RenewalResponse{
			MSISDN:        msisdn,
			ProductID:     product.ProductId,
			Success:       false,
			Status:        "OPT_IN_FAILED",
			OptOutStatus:  "SUCCESS",
			OptInStatus:   "FAILED",
			BillingStatus: "PENDING",
			Error:         optInErr.Error(),
		}, fmt.Errorf("opt-in failed: %w", optInErr)
	}

	cycle.OptInStatus = "SUCCESS"
	optInTime := time.Now()
	cycle.OptInTime = &optInTime
	cycle.BillingStatus = "PENDING"

	// Save the complete renewal cycle
	if err := r.SaveRenewalCycle(ctx, cycle); err != nil {
		r.logger.Error("Failed to save renewal cycle", zap.Error(err))
	}

	r.logger.Info("Renewal cycle completed successfully",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId),
		zap.Duration("totalTime", time.Since(cycle.CreatedAt)))

	return &domain.RenewalResponse{
		MSISDN:        msisdn,
		ProductID:     product.ProductId,
		Success:       true,
		Status:        "SUCCESS",
		CycleID:       cycle.ID,
		OptOutStatus:  cycle.OptOutStatus,
		OptInStatus:   cycle.OptInStatus,
		BillingStatus: cycle.BillingStatus,
		Message:       "Renewal cycle completed successfully",
	}, nil
}

// OptOutForRenewal handles the unsubscription part of renewal
func (r *RenewalService) OptOutForRenewal(ctx context.Context, msisdn string, product *domain.Product, cycle *domain.RenewalCycle) error {
	txId := uuid.New().String()

	// Convert product ID to int
	productID, err := strconv.Atoi(product.ProductId)
	if err != nil {
		return fmt.Errorf("invalid product ID: %w", err)
	}

	// Create UNSUBSCRIBE MT request
	mtReq := domain.MTRequest{
		ProductID:          productID,
		PricepointID:       product.PricePointId,
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		SubKeyword:         "STOP",
		Context:            "Unsubscription",
		MCC:                "620",
		MNC:                "03",
		EntryChannel:       "SYSTEM_RENEWAL",
		LargeAccount:       product.ShortCode,
		MoTransactionUUID:  txId,
		SendDate:           time.Now().Format(time.RFC3339),
	}

	r.logger.Debug("Sending opt-out request",
		zap.String("msisdn", msisdn),
		zap.String("transactionId", txId))

	// Use the subscription service to send the MT request
	response, err := r.subscriptionService.SendMT(mtReq, r.subscriptionService.config.Application.TIMWE.Realm, "SYSTEM")
	if err != nil {
		return fmt.Errorf("opt-out MT request failed: %w", err)
	}

	// Check response
	if response.Code != "0" && response.Code != "200" && response.Code != "SUCCESS" {
		return fmt.Errorf("opt-out response error: code=%s, message=%s",
			response.Code, response.Message)
	}

	// Update local subscription status
	r.UpdateSubscriptionStatus(ctx, msisdn, product.ProductId, "PENDING_RENEWAL")

	return nil
}

// OptInForRenewal handles the resubscription part of renewal
func (r *RenewalService) OptInForRenewal(ctx context.Context, msisdn string, product *domain.Product, entryChannel string, cycle *domain.RenewalCycle) error {
	txId := uuid.New().String()
	keyword := r.subscriptionService.generateThreeLetterKeyword(product.Name)

	// Convert product ID to int
	productID, err := strconv.Atoi(product.ProductId)
	if err != nil {
		return fmt.Errorf("invalid product ID: %w", err)
	}

	// Create SUBSCRIBE MT request
	mtReq := domain.MTRequest{
		ProductID:          productID,
		PricepointID:       product.PricePointId,
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		SubKeyword:         keyword,
		Context:            "Subscription",
		MCC:                "620",
		MNC:                "03",
		EntryChannel:       entryChannel,
		LargeAccount:       product.ShortCode,
		MoTransactionUUID:  txId,
		SendDate:           time.Now().Format(time.RFC3339),
	}

	r.logger.Debug("Sending opt-in request",
		zap.String("msisdn", msisdn),
		zap.String("transactionId", txId))

	response, err := r.subscriptionService.SendMT(mtReq, r.subscriptionService.config.Application.TIMWE.Realm, entryChannel)
	if err != nil {
		return fmt.Errorf("opt-in MT request failed: %w", err)
	}

	// Handle different response scenarios
	if r.isSubscriptionAlreadyActive(response) {
		r.logger.Warn("Subscription already active after opt-out",
			zap.String("msisdn", msisdn))
		// This shouldn't happen but handle gracefully
		return nil
	}

	if r.isSubscriptionWaitingForCharging(response) {
		r.logger.Info("Resubscription successful, waiting for charging",
			zap.String("msisdn", msisdn),
			zap.String("transactionId", txId))

		// Update subscription record
		r.UpdateSubscriptionStatus(ctx, msisdn, product.ProductId, "ACTIVE_WAITING_CHARGE")
		cycle.BillingStatus = "WAITING_CHARGE"
		return nil
	}

	// Check for errors
	if response.Code != "0" && response.Code != "200" && response.Code != "SUCCESS" {
		return fmt.Errorf("opt-in response error: code=%s, message=%s",
			response.Code, response.Message)
	}

	return nil
}

// EvaluateChurnPolicy determines what action to take for a subscription
func (r *RenewalService) EvaluateChurnPolicy(ctx context.Context, msisdn string, productID string) domain.ChurnAction {
	// Get subscription details - verify subscription exists
	_, err := r.repo.GetSubscription(msisdn, productID)
	if err != nil {
		r.logger.Error("Failed to get subscription for churn evaluation", zap.Error(err))
		return domain.ActionNoAction
	}

	// Get payment history - this needs to be implemented in repository
	lastPayment, err := r.repo.GetLastSuccessfulPayment(msisdn, productID)
	if err != nil {
		r.logger.Error("Failed to get last payment", zap.Error(err))
		return domain.ActionNoAction
	}

	// Calculate hours since last payment
	hoursSincePayment := 0.0
	if lastPayment != nil {
		hoursSincePayment = time.Since(*lastPayment).Hours()
	}

	// Get renewal attempts count
	renewalAttempts, err := r.repo.GetRenewalAttemptsCount(msisdn, productID,
		time.Now().Add(-time.Duration(r.config.ChurnPolicy.MaxHoursWithoutPayment)*time.Hour))
	if err != nil {
		r.logger.Error("Failed to get renewal attempts", zap.Error(err))
		renewalAttempts = 0
	}

	// Safety check to prevent mass churning
	if r.config.ChurnPolicy.SafeMode {
		dailyChurnCount, _ := r.repo.GetDailyChurnCount(time.Now())
		if dailyChurnCount > 1000 { // Safety threshold
			r.logger.Warn("Daily churn limit reached in safe mode",
				zap.Int("count", dailyChurnCount))
			return domain.ActionNoAction
		}
	}

	// Evaluation logic
	r.logger.Debug("Evaluating churn policy",
		zap.String("msisdn", msisdn),
		zap.Float64("hoursSincePayment", hoursSincePayment),
		zap.Int("renewalAttempts", renewalAttempts))

	// If payment is recent, no action needed
	if hoursSincePayment <= float64(r.config.ChurnPolicy.GracePeriodHours) {
		return domain.ActionNoAction
	}

	// If in grace period, just monitor
	if hoursSincePayment <= float64(r.config.ChurnPolicy.GracePeriodHours) {
		return domain.ActionGracePeriod
	}

	// If beyond max hours without payment
	if hoursSincePayment > float64(r.config.ChurnPolicy.MaxHoursWithoutPayment) {
		// Check if we've exhausted renewal attempts
		if renewalAttempts >= r.config.ChurnPolicy.MaxRenewalAttempts {
			r.logger.Info("Subscription should be churned",
				zap.String("msisdn", msisdn),
				zap.String("reason", "max_attempts_exceeded"))
			return domain.ActionChurn
		}

		// Check time since last attempt
		lastAttempt, _ := r.repo.GetLastRenewalAttempt(msisdn, productID)
		if lastAttempt != nil {
			hoursSinceLastAttempt := time.Since(*lastAttempt).Hours()
			if hoursSinceLastAttempt >= float64(r.config.ChurnPolicy.RetryIntervalHours) {
				return domain.ActionAttemptRenewal
			}
		} else {
			// No previous attempts, try renewal
			return domain.ActionAttemptRenewal
		}
	}

	// Default: attempt renewal if payment is overdue
	if hoursSincePayment > float64(r.config.ChurnPolicy.GracePeriodHours) {
		return domain.ActionAttemptRenewal
	}

	return domain.ActionNoAction
}

// ChurnSubscription permanently unsubscribes a user
func (r *RenewalService) ChurnSubscription(ctx context.Context, msisdn string, productID string, reason string) error {
	r.logger.Info("Churning subscription",
		zap.String("msisdn", msisdn),
		zap.String("productId", productID),
		zap.String("reason", reason))

	// Get product details
	product, err := r.productRepo.GetProduct(productID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	// Convert product ID to int
	productIDInt, err := strconv.Atoi(productID)
	if err != nil {
		return fmt.Errorf("invalid product ID: %w", err)
	}

	// Send final unsubscribe
	txId := uuid.New().String()
	mtReq := domain.MTRequest{
		ProductID:          productIDInt,
		PricepointID:       product.PricePointId,
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		SubKeyword:         "STOP",
		Context:            "Unsubscription",
		MCC:                "620",
		MNC:                "03",
		EntryChannel:       "SYSTEM_CHURN",
		LargeAccount:       product.ShortCode,
		MoTransactionUUID:  txId,
		SendDate:           time.Now().Format(time.RFC3339),
	}

	_, err = r.subscriptionService.SendMT(mtReq, r.subscriptionService.config.Application.TIMWE.Realm, "SYSTEM")
	if err != nil {
		r.logger.Error("Failed to send churn request", zap.Error(err))
		// Continue with local churn even if TIMWE fails
	}

	// Update database
	churnTime := time.Now()
	if err := r.repo.ChurnSubscription(msisdn, productID, reason, churnTime); err != nil {
		return fmt.Errorf("failed to update churn status: %w", err)
	}

	// Log to churn tracking table
	lastPayment, _ := r.repo.GetLastSuccessfulPayment(msisdn, productID)
	renewalAttempts, _ := r.repo.GetRenewalAttemptsCount(msisdn, productID, time.Now().AddDate(0, 0, -7))
	
	hoursWithoutPayment := 0
	if lastPayment != nil {
		hoursWithoutPayment = int(time.Since(*lastPayment).Hours())
	}

	churnRecord := &domain.ChurnRecord{
		MSISDN:               msisdn,
		ProductID:            productID,
		Reason:               reason,
		ChurnedAt:            churnTime,
		LastPaymentDate:      lastPayment,
		HoursWithoutPayment:  hoursWithoutPayment,
		TotalRenewalAttempts: renewalAttempts,
		CreatedAt:            time.Now(),
	}

	if err := r.repo.CreateChurnRecord(churnRecord); err != nil {
		r.logger.Error("Failed to create churn record", zap.Error(err))
	}

	return nil
}

// Helper methods

func (r *RenewalService) HandleFailedOptIn(ctx context.Context, msisdn string, product *domain.Product, cycle *domain.RenewalCycle) {
	r.logger.Error("CRITICAL: User unsubscribed but resubscription failed",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	// Add to priority retry queue
	r.AddToPriorityRetryQueue(ctx, msisdn, product, "FAILED_OPTIN")

	// Update status
	r.UpdateSubscriptionStatus(ctx, msisdn, product.ProductId, "FAILED_RENEWAL")
}

func (r *RenewalService) AddToPriorityRetryQueue(ctx context.Context, msisdn string, product *domain.Product, reason string) {
	retryItem := &domain.PriorityRetryQueue{
		MSISDN:    msisdn,
		ProductID: product.ProductId,
		Reason:    reason,
		Priority:  1,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	nextRetry := time.Now().Add(5 * time.Minute)
	retryItem.NextRetryAt = &nextRetry

	if err := r.repo.AddToPriorityRetryQueue(retryItem); err != nil {
		r.logger.Error("Failed to add to priority retry queue", zap.Error(err))
	}
}

func (r *RenewalService) SaveRenewalCycle(ctx context.Context, cycle *domain.RenewalCycle) error {
	return r.repo.SaveRenewalCycle(cycle)
}

func (r *RenewalService) UpdateSubscriptionStatus(ctx context.Context, msisdn string, productID string, status string) {
	if err := r.repo.UpdateSubscriptionStatus(msisdn, productID, status); err != nil {
		r.logger.Error("Failed to update subscription status",
			zap.String("msisdn", msisdn),
			zap.String("status", status),
			zap.Error(err))
	}
}

func (r *RenewalService) isSubscriptionAlreadyActive(response *domain.MTResponse) bool {
	if response.ResponseData == nil {
		return false
	}
	
	if result, ok := response.ResponseData["subscriptionResult"].(string); ok {
		return result == domain.SubscriptionResultOptinAlreadyActive
	}
	return false
}

func (r *RenewalService) isSubscriptionWaitingForCharging(response *domain.MTResponse) bool {
	if response.ResponseData == nil {
		return false
	}
	
	if result, ok := response.ResponseData["subscriptionResult"].(string); ok {
		return result == domain.SubscriptionResultOptinActiveWaitCharging
	}
	return false
}

// GetRenewalStatistics returns renewal statistics for the given hours back period
func (r *RenewalService) GetRenewalStatistics(ctx context.Context, hoursBack int) (*domain.RenewalMetrics, error) {
	// Calculate the time threshold
	since := time.Now().Add(-time.Duration(hoursBack) * time.Hour)

	metrics := &domain.RenewalMetrics{
		LastRunTime: time.Now(),
	}

	// Get total renewals processed in the period
	// This is a simplified implementation - you may want to add more detailed tracking
	renewalAttempts, err := r.repo.GetRenewalAttemptsCount("", "", since)
	if err != nil {
		r.logger.Warn("Failed to get renewal attempts count", zap.Error(err))
	} else {
		metrics.TotalProcessed = int64(renewalAttempts)
	}

	// Get churn count
	churnCount, err := r.repo.GetDailyChurnCount(time.Now())
	if err != nil {
		r.logger.Warn("Failed to get churn count", zap.Error(err))
	} else {
		metrics.ChurnedSubscriptions = int64(churnCount)
	}

	// Calculate success rate
	if metrics.TotalProcessed > 0 {
		metrics.SuccessRate = float64(metrics.SuccessfulRenewals) / float64(metrics.TotalProcessed) * 100
	}

	return metrics, nil
}

// GetChurnCandidates returns subscriptions that are candidates for churning
func (r *RenewalService) GetChurnCandidates(ctx context.Context, maxDays int, maxAttempts int, limit int) ([]*domain.SubscriptionWithRenewalInfo, error) {
	// Convert days to hours for the query
	maxHours := maxDays * 24

	// Use the repository method to get subscriptions needing renewal
	subs, err := r.repo.GetSubscriptionsNeedingRenewal(maxHours, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get churn candidates: %w", err)
	}

	// Filter to only those that have exceeded max attempts
	var candidates []*domain.SubscriptionWithRenewalInfo
	for _, sub := range subs {
		if sub.TotalRenewalAttempts >= maxAttempts {
			candidates = append(candidates, sub)
		}
	}

	return candidates, nil
}

// GetProduct returns a product by its ID
func (r *RenewalService) GetProduct(ctx context.Context, productID string) (*domain.Product, error) {
	return r.productRepo.GetProduct(productID)
}
