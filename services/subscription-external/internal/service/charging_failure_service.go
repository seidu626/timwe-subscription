// charging_failure_service.go - Service for handling charging failures using notifications
// File: internal/service/charging_failure_service.go
// Based on FINAL_CHARGING_STRATEGY.md

package service

import (
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

// ChargingFailureService handles charging failure operations
type ChargingFailureService struct {
	repo   repository.SubscriptionRepositoryInterface
	logger *zap.Logger
}

// NewChargingFailureService creates a new charging failure service
func NewChargingFailureService(repo repository.SubscriptionRepositoryInterface, logger *zap.Logger) *ChargingFailureService {
	return &ChargingFailureService{
		repo:   repo,
		logger: logger,
	}
}

// GetChargingFailures retrieves subscriptions with charging issues
func (s *ChargingFailureService) GetChargingFailures(filter repository.ChargingFailureFilter) ([]repository.ChargingFailedSubscription, error) {
	s.logger.Info("Fetching charging failures",
		zap.Int("limit", filter.Limit),
		zap.Int("offset", filter.Offset),
		zap.Int("days_threshold", filter.DaysThreshold))

	subscriptions, err := s.repo.FetchChargingFailedSubscriptions(filter)
	if err != nil {
		s.logger.Error("Failed to fetch charging failures", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch charging failures: %w", err)
	}

	s.logger.Info("Successfully fetched charging failures",
		zap.Int("count", len(subscriptions)))

	return subscriptions, nil
}

// GetChargingFailureCount returns the total count of subscriptions with charging issues
func (s *ChargingFailureService) GetChargingFailureCount(filter repository.ChargingFailureFilter) (int64, error) {
	s.logger.Info("Getting charging failure count",
		zap.Int("days_threshold", filter.DaysThreshold))

	count, err := s.repo.GetChargingFailureCount(filter)
	if err != nil {
		s.logger.Error("Failed to get charging failure count", zap.Error(err))
		return 0, fmt.Errorf("failed to get charging failure count: %w", err)
	}

	s.logger.Info("Successfully got charging failure count",
		zap.Int64("count", count))

	return count, nil
}

// GetChargingFailureStats returns statistics about charging failures
func (s *ChargingFailureService) GetChargingFailureStats() (map[string]interface{}, error) {
	s.logger.Info("Getting charging failure statistics")

	stats, err := s.repo.GetChargingFailureStats()
	if err != nil {
		s.logger.Error("Failed to get charging failure stats", zap.Error(err))
		return nil, fmt.Errorf("failed to get charging failure stats: %w", err)
	}

	s.logger.Info("Successfully got charging failure statistics",
		zap.Any("stats", stats))

	return stats, nil
}

// GetChargingFailureSummary returns a summary view of charging failures
func (s *ChargingFailureService) GetChargingFailureSummary() (map[string]interface{}, error) {
	s.logger.Info("Getting charging failure summary")

	summary, err := s.repo.GetChargingFailureSummary()
	if err != nil {
		s.logger.Error("Failed to get charging failure summary", zap.Error(err))
		return nil, fmt.Errorf("failed to get charging failure summary: %w", err)
	}

	s.logger.Info("Successfully got charging failure summary",
		zap.Any("summary", summary))

	return summary, nil
}

// GetChargingFailureByMSISDN retrieves charging failure information for a specific MSISDN
func (s *ChargingFailureService) GetChargingFailureByMSISDN(msisdn string, productID int) (*repository.ChargingFailedSubscription, error) {
	s.logger.Info("Getting charging failure by MSISDN",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID))

	failure, err := s.repo.GetChargingFailureByMSISDN(msisdn, productID)
	if err != nil {
		s.logger.Error("Failed to get charging failure by MSISDN", zap.Error(err))
		return nil, fmt.Errorf("failed to get charging failure by MSISDN: %w", err)
	}

	if failure == nil {
		s.logger.Info("No charging failure found for MSISDN",
			zap.String("msisdn", msisdn))
		return nil, nil
	}

	s.logger.Info("Successfully got charging failure by MSISDN",
		zap.String("msisdn", msisdn),
		zap.String("charging_status", failure.ChargingStatus))

	return failure, nil
}

// UpdateChargingHealthStatus updates the charging health status for a subscription
func (s *ChargingFailureService) UpdateChargingHealthStatus(subscriptionID int, status string, reason string) error {
	s.logger.Info("Updating charging health status",
		zap.Int("subscription_id", subscriptionID),
		zap.String("status", status),
		zap.String("reason", reason))

	err := s.repo.UpdateChargingHealthStatus(subscriptionID, status, reason)
	if err != nil {
		s.logger.Error("Failed to update charging health status", zap.Error(err))
		return fmt.Errorf("failed to update charging health status: %w", err)
	}

	s.logger.Info("Successfully updated charging health status",
		zap.Int("subscription_id", subscriptionID),
		zap.String("status", status))

	return nil
}

// MarkChargingFailureAsProcessed marks a charging failure as processed
func (s *ChargingFailureService) MarkChargingFailureAsProcessed(subscriptionID int, status string) error {
	s.logger.Info("Marking charging failure as processed",
		zap.Int("subscription_id", subscriptionID),
		zap.String("status", status))

	err := s.repo.MarkChargingFailureAsProcessed(subscriptionID, status)
	if err != nil {
		s.logger.Error("Failed to mark charging failure as processed", zap.Error(err))
		return fmt.Errorf("failed to mark charging failure as processed: %w", err)
	}

	s.logger.Info("Successfully marked charging failure as processed",
		zap.Int("subscription_id", subscriptionID),
		zap.String("status", status))

	return nil
}

// ProcessChargingFailures processes a batch of charging failures for resubscription
func (s *ChargingFailureService) ProcessChargingFailures(batchSize int, daysThreshold int) error {
	s.logger.Info("Starting charging failure processing",
		zap.Int("batch_size", batchSize),
		zap.Int("days_threshold", daysThreshold))

	filter := repository.ChargingFailureFilter{
		Limit:            batchSize,
		Offset:           0,
		DaysThreshold:    daysThreshold,
		ExcludeProcessed: true,
	}

	// Get charging failures
	subscriptions, err := s.repo.FetchChargingFailedSubscriptions(filter)
	if err != nil {
		s.logger.Error("Failed to fetch charging failures for processing", zap.Error(err))
		return fmt.Errorf("failed to fetch charging failures: %w", err)
	}

	if len(subscriptions) == 0 {
		s.logger.Info("No charging failures to process")
		return nil
	}

	s.logger.Info("Processing charging failures",
		zap.Int("count", len(subscriptions)))

	// Process each subscription
	for _, subscription := range subscriptions {
		s.logger.Info("Processing charging failure",
			zap.Int("subscription_id", subscription.ID),
			zap.String("msisdn", subscription.MSISDN),
			zap.String("charging_status", subscription.ChargingStatus))

		// Mark as in progress
		err := s.repo.UpdateChargingHealthStatus(subscription.ID, "IN_PROGRESS", "Processing for resubscription")
		if err != nil {
			s.logger.Error("Failed to update charging health status to IN_PROGRESS", zap.Error(err))
			continue
		}

		// TODO: Implement actual resubscription logic here
		// For now, just mark as processed
		err = s.repo.MarkChargingFailureAsProcessed(subscription.ID, "processed")
		if err != nil {
			s.logger.Error("Failed to mark charging failure as processed", zap.Error(err))
			continue
		}

		s.logger.Info("Successfully processed charging failure",
			zap.Int("subscription_id", subscription.ID),
			zap.String("msisdn", subscription.MSISDN))
	}

	s.logger.Info("Completed charging failure processing",
		zap.Int("processed_count", len(subscriptions)))

	return nil
}

// GetChargingFailureMetrics returns key metrics for monitoring
func (s *ChargingFailureService) GetChargingFailureMetrics() (map[string]interface{}, error) {
	s.logger.Info("Getting charging failure metrics")

	// Get summary
	summary, err := s.repo.GetChargingFailureSummary()
	if err != nil {
		s.logger.Error("Failed to get charging failure summary for metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get charging failure summary: %w", err)
	}

	// Get stats
	stats, err := s.repo.GetChargingFailureStats()
	if err != nil {
		s.logger.Error("Failed to get charging failure stats for metrics", zap.Error(err))
		// Continue without stats rather than failing completely
		stats = make(map[string]interface{})
	}

	// Get current count
	filter := repository.ChargingFailureFilter{
		ExcludeProcessed: true,
		DaysThreshold:    30,
	}
	currentCount, err := s.repo.GetChargingFailureCount(filter)
	if err != nil {
		s.logger.Error("Failed to get current charging failure count for metrics", zap.Error(err))
		currentCount = 0
	}

	metrics := map[string]interface{}{
		"timestamp":         time.Now().Format(time.RFC3339),
		"summary":           summary,
		"detailed_stats":    stats,
		"current_failures":  currentCount,
		"processing_status": "active",
		"last_updated":      time.Now().Format(time.RFC3339),
	}

	s.logger.Info("Successfully got charging failure metrics",
		zap.Int64("current_failures", currentCount))

	return metrics, nil
}
