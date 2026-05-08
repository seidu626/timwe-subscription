package utils

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"

	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"
)

// Example implementation showing how to use the enhanced MSISDN generator
// with real Tigo userbase data

func main() {
	fmt.Println("MSISDN Generator Example Usage")
	fmt.Println("==============================")

	// Step 1: Setup logging
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Step 2: Setup configuration
	cfg := createSampleConfig()

	// Step 3: Create a mock repository (in production, use real repository)
	repo := &MockRepository{}

	// Step 4: Create optimized MSISDN generator
	generator := NewOptimizedMSISDNGenerator(
		nil, // no bloom filter for this example
		repo,
		logger,
		100, // batch size
		10,  // max concurrent
	)

	// Step 5: Generate single MSISDN
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Generating single MSISDN...")
	msisdn, err := generator.GenerateRandomMSISDNOptimized(context.Background(), "tigo", cfg)
	if err != nil {
		log.Fatalf("Failed to generate MSISDN: %v", err)
	}
	fmt.Printf("✓ Generated MSISDN: %s\n", msisdn)

	// Step 6: Generate batch of MSISDNs
	fmt.Println("\nGenerating batch of 10 MSISDNs...")
	batch, err := generator.GenerateBatchMSISDNSOptimized(context.Background(), "tigo", 10, cfg)
	if err != nil {
		log.Fatalf("Failed to generate batch: %v", err)
	}

	fmt.Println("✓ Generated MSISDNs:")
	for i, m := range batch {
		fmt.Printf("  %2d. %s\n", i+1, m)
	}

	// Step 7: Generate MSISDNs for different telcos
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Generating MSISDNs for different telcos...")
	telcos := []string{"tigo", "airtel", "mtn", "vodafone"}

	for _, telco := range telcos {
		msisdn, err := generator.GenerateRandomMSISDNOptimized(context.Background(), telco, cfg)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", telco, err)
		} else {
			fmt.Printf("  ✓ %s: %s\n", telco, msisdn)
		}
	}

	// Step 8: Demonstrate weighted generation
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Testing weighted prefix distribution (100 generations)...")
	prefixCounts := make(map[string]int)

	for i := 0; i < 100; i++ {
		msisdn, err := generator.GenerateRandomMSISDNOptimized(context.Background(), "tigo", cfg)
		if err == nil && len(msisdn) >= 6 {
			prefix := msisdn[:6]
			prefixCounts[prefix]++
		}
	}

	fmt.Println("✓ Prefix distribution from generated numbers:")
	for prefix, count := range prefixCounts {
		fmt.Printf("  %s: %d (%.1f%%)\n", prefix, count, float64(count))
	}

	// Step 9: Performance test
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Performance test: Generating 1000 MSISDNs...")
	start := time.Now()

	largeBatch, err := generator.GenerateBatchMSISDNSOptimized(context.Background(), "tigo", 1000, cfg)
	if err != nil {
		log.Fatalf("Failed to generate large batch: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("✓ Generated %d MSISDNs in %v\n", len(largeBatch), elapsed)
	fmt.Printf("  Average time per MSISDN: %v\n", elapsed/time.Duration(len(largeBatch)))

	// Check uniqueness
	uniqueMap := make(map[string]bool)
	for _, m := range largeBatch {
		uniqueMap[m] = true
	}
	fmt.Printf("  Unique MSISDNs: %d (%.1f%%)\n",
		len(uniqueMap), float64(len(uniqueMap))/float64(len(largeBatch))*100)

	// Step 10: Show generator statistics
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Generator Statistics:")
	stats := generator.GetStats()
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("✓ MSISDN Generator Test Complete!")
}

// Helper function to get top N prefixes by count
type PrefixCount struct {
	Prefix string
	Count  int
}

func getTopPrefixes(distribution map[string]int, n int) []PrefixCount {
	// Convert map to slice for sorting
	prefixes := make([]PrefixCount, 0, len(distribution))
	for prefix, count := range distribution {
		prefixes = append(prefixes, PrefixCount{Prefix: prefix, Count: count})
	}

	// Sort by count (descending)
	for i := 0; i < len(prefixes)-1; i++ {
		for j := i + 1; j < len(prefixes); j++ {
			if prefixes[j].Count > prefixes[i].Count {
				prefixes[i], prefixes[j] = prefixes[j], prefixes[i]
			}
		}
	}

	// Return top N
	if n > len(prefixes) {
		n = len(prefixes)
	}
	return prefixes[:n]
}

// createSampleConfig creates a sample configuration for testing
func createSampleConfig() *config.Config {
	cfg := &config.Config{}

	// Set basic application settings
	cfg.Application.Environment = config.DEVELOPMENT
	cfg.Application.Port = 8083
	cfg.Application.AllowedOrigins = []string{"http://localhost:4200"}

	// Set telco prefixes
	cfg.Application.TelcoPrefixes = map[string][]string{
		"AirtelTigo": {
			"233278", "233203", "233578", "233242", "233307",
			"233245", "233247", "233576", "233271", "233273",
		},
	}

	// Set log configuration
	cfg.Application.Log.Path = "/var/log/app.log"
	cfg.Application.Log.Rolling.Enabled = true
	cfg.Application.Log.Rolling.MaxSize = 100
	cfg.Application.Log.Rolling.MaxAge = 30
	cfg.Application.Log.Rolling.MaxBackups = 10
	cfg.Application.Log.Rolling.Compress = true
	cfg.Application.Log.Rolling.CompressThreshold = 10
	cfg.Application.Log.Rolling.LocalTime = true

	return cfg
}

// MockRepository is a simple mock implementation for testing
type MockRepository struct{}

func (m *MockRepository) LoadExclusionList() (map[string]bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockRepository) GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockRepository) InsertUserRecords(ctx context.Context, records []*domain.UserBase) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockRepository) IsExcludedUser(msisdn string) (bool, error) {
	// For testing, consider some numbers as excluded (Premier/Staff/Blacklisted)
	if strings.HasSuffix(msisdn, "000000") || strings.HasSuffix(msisdn, "111111") {
		return true, nil
	}
	return false, nil
}

func (m *MockRepository) GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	// For testing, consider some patterns as invalid
	invalid := []string{}
	for _, msisdn := range msisdns {
		if strings.HasSuffix(msisdn, "999999") {
			invalid = append(invalid, msisdn)
		}
	}
	return invalid, nil
}

func (m *MockRepository) GetBlacklistedMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	// For testing, consider some patterns as blacklisted
	blacklisted := []string{}
	for _, msisdn := range msisdns {
		if strings.HasSuffix(msisdn, "888888") {
			blacklisted = append(blacklisted, msisdn)
		}
	}
	return blacklisted, nil
}

func (m *MockRepository) FilterMSISDNS(msisdns []string) ([]string, error) {
	// For testing, return all MSISDNs that are not excluded
	valid := []string{}
	for _, msisdn := range msisdns {
		isExcluded, _ := m.IsExcludedUser(msisdn)
		if !isExcluded {
			valid = append(valid, msisdn)
		}
	}
	return valid, nil
}

// GetInvalidMSISDNSOptimized is an optimized version with caching and better queries
func (m *MockRepository) GetInvalidMSISDNSOptimized(ctx context.Context, msisdns []string) ([]string, error) {
	// For testing, use the same logic as GetInvalidMSISDNS
	return m.GetInvalidMSISDNS(ctx, msisdns)
}

// GetInvalidMSISDNSFast is the fastest version for single MSISDN lookups
func (m *MockRepository) GetInvalidMSISDNSFast(ctx context.Context, msisdn string) (bool, error) {
	// For testing, consider some patterns as invalid
	if strings.HasSuffix(msisdn, "999999") {
		return true, nil
	}
	return false, nil
}

// GetInvalidMSISDNSStats returns statistics about the invalid_msisdn_logs table
func (m *MockRepository) GetInvalidMSISDNSStats(ctx context.Context) (map[string]interface{}, error) {
	// For testing, return mock statistics
	return map[string]interface{}{
		"total_records":  1000,
		"unique_msisdns": 950,
		"table_size":     "50 MB",
		"index_count":    5,
		"last_updated":   time.Now().Format(time.RFC3339),
	}, nil
}
