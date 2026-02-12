package repository

import (
	"database/sql"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

// DBGetter is an interface for repositories that can provide database access
type DBGetter interface {
	GetDB() *sql.DB
}

// SubscriptionRepositoryInterface defines the interface for SubscriptionRepository
type SubscriptionRepositoryInterface interface {
	CreateSubscription(request *domain.SubscriptionRequest) error
	CreateNotification(notification *domain.NotificationRequest) error
	CreateInvalidMSISDNLog(log *domain.InvalidMSISDNLog) error
	CheckSubscriptionExists(msisdn string, productId int) (bool, error)
	CheckRenewalNotificationExists(msisdn string, productId int) (bool, error)
	// HasAnySubscription checks if any subscriptions exist for the given MSISDN regardless of product
	HasAnySubscription(msisdn string) (bool, error)
	// FindAndRemoveSubscription finds and removes/deactivates a subscription for the given MSISDN and product
	FindAndRemoveSubscription(msisdn string, productId int) error
	// DeleteSubscriptionRecord completely removes a subscription record for the given MSISDN and product
	DeleteSubscriptionRecord(msisdn string) error
	GenerateCacheKey(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) string
	// FetchActiveMsisdnsMissingSomeProducts returns active MSISDNs that are missing at least one of the provided product IDs using offset/limit windowing
	FetchActiveMsisdnsMissingSomeProducts(productIds []int, offset int, limit int) ([]string, error)
	// FetchActiveMsisdnsWithProductsWindow returns active MSISDNs that have any of the provided product IDs using offset/limit windowing
	FetchActiveMsisdnsWithProductsWindow(productIds []int, offset int, limit int) ([]string, error)

	// Background monitor extensions
	FetchNotificationsWindow(ntype string, since time.Time, afterId int64, limit int) ([]NotificationRow, error)
	FetchSubscriptionsNeedingRenewal(cutoff time.Time, afterId int64, limit int) ([]NotificationRow, error)
	UpsertSubscriptionStatus(msisdn string, productId int, status string) error
	// Enhanced: Fetch unprocessed opt-out notifications to prevent duplicates
	FetchUnprocessedOptoutNotifications(since time.Time, afterId int64, limit int) ([]NotificationRow, error)
	// Enhanced: Get subscription by MSISDN and product ID
	GetSubscriptionByMSISDNAndProduct(msisdn string, productID int) (*domain.Subscription, error)
	// Enhanced: Get last opt-in notification time for MSISDN + product
	GetLastOptinNotificationTime(msisdn string, productID int) (*time.Time, error)
	// Enhanced: Fetch ghost subscriptions (subscriptions without opt-in notifications)
	FetchGhostSubscriptions(cutoff time.Time, afterId int64, limit int) ([]NotificationRow, error)

	// Charging failure methods using notifications-based approach
	FetchChargingFailedSubscriptions(filter ChargingFailureFilter) ([]ChargingFailedSubscription, error)
	GetChargingFailureCount(filter ChargingFailureFilter) (int64, error)
	GetChargingFailureStats() (map[string]interface{}, error)
	GetChargingFailureSummary() (map[string]interface{}, error)
	GetChargingFailureByMSISDN(msisdn string, productID int) (*ChargingFailedSubscription, error)
	UpdateChargingHealthStatus(subscriptionID int, status string, reason string) error
	MarkChargingFailureAsProcessed(subscriptionID int, status string) error

	// Subscription count methods
	GetTotalSubscriptionsCount() (int64, error)

	// Renewal-related methods
	GetSubscription(msisdn string, productID string) (*domain.SubscriptionWithRenewalInfo, error)
	GetLastSuccessfulPayment(msisdn string, productID string) (*time.Time, error)
	GetRenewalAttemptsCount(msisdn string, productID string, since time.Time) (int, error)
	GetDailyChurnCount(date time.Time) (int, error)
	GetLastRenewalAttempt(msisdn string, productID string) (*time.Time, error)
	ChurnSubscription(msisdn string, productID string, reason string, churnTime time.Time) error
	CreateChurnRecord(record *domain.ChurnRecord) error
	SaveRenewalCycle(cycle *domain.RenewalCycle) error
	UpdateSubscriptionStatus(msisdn string, productID string, status string) error
	AddToPriorityRetryQueue(item *domain.PriorityRetryQueue) error
	IncrementRenewalAttempt(msisdn string, productID string) error
	GetSubscriptionsNeedingRenewal(hoursThreshold int, limit int) ([]*domain.SubscriptionWithRenewalInfo, error)
	SaveRenewalMetrics(metrics *domain.RenewalMetrics) error
	GetDuePriorityRetryItems(limit int) ([]*domain.PriorityRetryQueue, error)
	UpdatePriorityRetryItem(item *domain.PriorityRetryQueue) error
}

type NotificationRow struct {
	ID           int
	MSISDN       string
	ProductID    int
	EntryChannel string
	CreatedAt    time.Time
	Type         string
}
