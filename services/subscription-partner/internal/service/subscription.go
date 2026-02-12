package service

import (
	"errors"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/repository"
	"log"
	"strconv"
	"time"
)

type SubscriptionService struct {
	repo   *repository.SubscriptionRepository
	config *config.Config
}

func NewSubscriptionService(repo *repository.SubscriptionRepository, cfg *config.Config) *SubscriptionService {
	return &SubscriptionService{repo: repo, config: cfg}
}

func (s *SubscriptionService) GetSubscriptions(filters map[string]string) (*domain.ListResponse, error) {
	// Parse filter values
	startDate, _ := time.Parse("2006-01-02", filters["startDate"])
	endDate, _ := time.Parse("2006-01-02", filters["endDate"])
	productId, _ := strconv.Atoi(filters["productId"])
	shortcode := filters["shortcode"]
	userIdentifier := filters["userIdentifier"]
	entryChannel := filters["entryChannel"]
	page, _ := strconv.Atoi(filters["page"])
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(filters["pageSize"])
	if pageSize < 1 {
		pageSize = 10
	}

	// Pass filters to the repository layer
	return s.repo.FetchSubscriptions(startDate, endDate, productId, shortcode, userIdentifier, entryChannel, page, pageSize)
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
