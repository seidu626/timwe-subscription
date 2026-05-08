package service

import (
	"context"
	"fmt"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
	"strconv"
	"strings"
)

type UserBaseService struct {
	repo   *repository.UserBaseRepository
	logger *zap.Logger
	config *config.Config
}

// NewUserBaseService initializes a new UserBaseService with the repository
func NewUserBaseService(logger *zap.Logger, repo *repository.UserBaseRepository, c *config.Config) *UserBaseService {
	return &UserBaseService{repo: repo, logger: logger, config: c}
}

// ValidateAndStoreUserRecords validates and stores a batch of UserBase records
// It ensures MSISDNS have the correct prefix and filters out duplicates
func (service *UserBaseService) ValidateAndStoreUserRecords(ctx context.Context, records []*domain.UserBase) error {
	// Normalize and validate MSISDNS and types
	var normalizedRecords []*domain.UserBase
	uniqueMSISDNS := make(map[string]bool) // Set to track unique MSISDNS in this batch

	for _, record := range records {
		// Normalize and validate UserIdentifier and Type
		record.Msisdn = strings.TrimSpace(record.Msisdn)
		record.Type = strings.TrimSpace(record.Type)

		if !service.validateMSISDN(&record.Msisdn) || !service.validateUserType(record.Type) {
			continue // Skip invalid records
		}

		// Check for duplicates within the uploaded batch
		if uniqueMSISDNS[record.Msisdn] {
			continue // Skip duplicate in the same upload batch
		}

		// Add to unique set and normalized list
		uniqueMSISDNS[record.Msisdn] = true
		normalizedRecords = append(normalizedRecords, record)
	}

	// Filter out existing MSISDNS
	msisdns := make([]string, len(normalizedRecords))
	for i, record := range normalizedRecords {
		msisdns[i] = record.Msisdn
	}

	existingMSISDNS, err := service.repo.GetExistingMSISDNS(ctx, msisdns)
	if err != nil {
		return fmt.Errorf("failed to check existing MSISDNS: %v", err)
	}

	existingSet := make(map[string]bool)
	for _, msisdn := range existingMSISDNS {
		existingSet[msisdn] = true
	}

	var uniqueRecords []*domain.UserBase
	for _, record := range normalizedRecords {
		if !existingSet[record.Msisdn] {
			uniqueRecords = append(uniqueRecords, record)
		}
	}

	// Insert unique records into the database
	err = service.repo.InsertUserRecords(ctx, uniqueRecords)
	if err != nil {
		return fmt.Errorf("failed to store records: %v", err)
	}

	return nil
}

// validateMSISDN ensures UserIdentifier has the "233" prefix
func (service *UserBaseService) validateMSISDN(msisdn *string) bool {
	if !strings.HasPrefix(*msisdn, "233") {
		*msisdn = "233" + strings.TrimLeft(*msisdn, "0") // trim leading 0s to avoid duplicating "233"
	}
	_, err := strconv.ParseInt(*msisdn, 10, 64)
	return err == nil && len(*msisdn) >= 10
}

// validateUserType checks if the user type is allowed
func (service *UserBaseService) validateUserType(userType string) bool {
	allowedUserTypes := map[string]bool{"Premier": true, "Staff": true, "Regular": true}
	return allowedUserTypes[userType]
}
