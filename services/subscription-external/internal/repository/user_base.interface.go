package repository

import (
	"context"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

// UserBaseRepositoryInterface defines the interface for UserBaseRepository
type UserBaseRepositoryInterface interface {
	IsExcludedUser(msisdn string) (bool, error)
	FilterMSISDNS(msisdns []string) ([]string, error)
	LoadExclusionList() (map[string]bool, error)
	GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error)
	InsertUserRecords(ctx context.Context, records []*domain.UserBase) error
	// GetInvalidMSISDNS checks if MSISDNs exist in the invalid_msisdn_logs table
	GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error)
	// GetInvalidMSISDNSOptimized is an optimized version with caching and better queries
	GetInvalidMSISDNSOptimized(ctx context.Context, msisdns []string) ([]string, error)
	// GetInvalidMSISDNSFast is the fastest version for single MSISDN lookups
	GetInvalidMSISDNSFast(ctx context.Context, msisdn string) (bool, error)
	// GetInvalidMSISDNSStats returns statistics about the invalid_msisdn_logs table
	GetInvalidMSISDNSStats(ctx context.Context) (map[string]interface{}, error)
	// GetBlacklistedMSISDNS checks if MSISDNs are blacklisted in the userbase table
	GetBlacklistedMSISDNS(ctx context.Context, msisdns []string) ([]string, error)
}
