package service

import (
	"errors"
	"fmt"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"log"
	"strconv"
	"strings"
	"time"
)

type SubscriptionService struct {
	repo   subscriptionRepository
	config *config.Config
}

type subscriptionRepository interface {
	FetchSubscriptions(tenantID, tenantKey string, startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error)
	ConfirmSubscription(request *domain.SubscriptionConfirmationRequest) error
	CreateSubscription(request *domain.SubscriptionRequest) error
	CreateNotification(notification *domain.NotificationRequest) error
	OptOutSubscription(request *domain.UnsubscriptionRequest) error
	GetSubscriptionStatus(request *domain.GetStatusRequest) (*domain.SubscriptionStatus, error)
}

func NewSubscriptionService(repo subscriptionRepository, cfg *config.Config) *SubscriptionService {
	return &SubscriptionService{repo: repo, config: cfg}
}

func (s *SubscriptionService) GetSubscriptions(filters map[string]string) (*domain.ListResponse, error) {
	// Parse filter values
	startDate := parseFilterDate(filters["startDate"], false)
	endDate := parseFilterDate(filters["endDate"], true)
	productId, _ := strconv.Atoi(filters["productId"])
	tenantID := strings.TrimSpace(filters["tenantId"])
	tenantKey := strings.TrimSpace(filters["tenantKey"])
	shortcode := filters["shortcode"]
	userIdentifier := filters["userIdentifier"]
	entryChannel := filters["entryChannel"]
	sortBy := filters["sort_by"]
	if sortBy == "" {
		sortBy = filters["sortBy"]
	}
	sortDir := filters["sort_dir"]
	if sortDir == "" {
		sortDir = filters["sortDir"]
	}
	page, _ := strconv.Atoi(filters["page"])
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(filters["pageSize"])
	if pageSize < 1 {
		pageSize = 10
	}

	// Pass filters to the repository layer
	listResponse, err := s.repo.FetchSubscriptions(tenantID, tenantKey, startDate, endDate, productId, shortcode, userIdentifier, entryChannel, sortBy, sortDir, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions failed (page=%d pageSize=%d): %w", page, pageSize, err)
	}

	return listResponse, nil
}

func (s *SubscriptionService) ProcessConfirmation(req *domain.SubscriptionConfirmationRequest) error {
	// Implement the logic for processing a confirmation request
	if req.PartnerRoleId == 0 || req.UserIdentifier == "" || req.TransactionAuthCode == "" {
		return errors.New("invalid confirmation request")
	}

	// Step 4: Save the new subscription
	if err := s.repo.ConfirmSubscription(req); err != nil {
		log.Printf("Error saving subscription: %v", err)
		return err
	}
	return nil
}

func (s *SubscriptionService) ProcessOptin(req *domain.SubscriptionRequest) error {
	if req.PartnerRoleId == 0 || req.UserIdentifier == "" {
		return errors.New("invalid subscription request")
	}

	// Step 4: Save the new subscription
	if err := s.repo.CreateSubscription(req); err != nil {
		log.Printf("Error saving subscription: %v", err)
		return err
	}

	return nil
}

func (s *SubscriptionService) ProcessOptout(req *domain.UnsubscriptionRequest) error {
	// Implement the logic for processing an opt-out request
	if req.PartnerRoleId == 0 || req.UserIdentifier == "" {
		return errors.New("invalid unsubscription request")
	}

	// Step 4: Save the new subscription
	if err := s.repo.OptOutSubscription(req); err != nil {
		log.Printf("Error saving subscription: %v", err)
		return err
	}

	return nil
}

func (s *SubscriptionService) ProcessStatus(req *domain.GetStatusRequest) (*domain.SubscriptionStatus, error) {
	// Implement the logic for processing a status request
	if req.PartnerRoleId == 0 || req.UserIdentifier == "" {
		return nil, errors.New("invalid status request")
	}

	// Step 4: Save the new subscription
	status, err := s.repo.GetSubscriptionStatus(req)
	if err != nil {
		log.Printf("Error saving subscription: %v", err)
		return nil, err
	}
	return status, nil
}

func (s *SubscriptionService) ProcessNotification(req *domain.NotificationRequest) error {
	if req == nil || req.Type == "" || req.MSISDN == "" {
		return errors.New("invalid notification request")
	}
	if err := s.repo.CreateNotification(req); err != nil {
		return err
	}
	return nil
}

// Helper function to validate the authentication token (mock implementation)
func (s *SubscriptionService) validateAuthToken(authToken string) bool {
	// Placeholder: Add logic to validate the token (e.g., decrypt and check timestamp validity)
	return len(authToken) > 0
}

// Helper function to validate the API key (mock implementation)
func (s *SubscriptionService) validateApiKey(apiKey string) bool {
	// Placeholder: Check if the API key matches what is expected (store these securely in practice)
	return apiKey == "expected-api-key"
}

func parseFilterDate(raw string, endOfDay bool) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	if len(raw) == len("2006-01-02") {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			if endOfDay {
				return t.Add(24*time.Hour - time.Nanosecond)
			}
			return t
		}
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}

	return time.Time{}
}
