package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

type UserBaseRepository struct {
	db     *sql.DB
	logger *zap.Logger
	redis  cached.RedisClient
	ctx    context.Context
}

// NewUserBaseRepository initializes a new UserBaseRepository with the database connection
func NewUserBaseRepository(db *sql.DB, logger *zap.Logger, client cached.RedisClient) *UserBaseRepository {
	return &UserBaseRepository{db: db,
		logger: logger,
		redis:  client,
		ctx:    context.Background(),
	}
}

// cacheKey generates a Redis key for storing UserIdentifier lookup results
func cacheKey(msisdn string) string {
	return fmt.Sprintf("userbase:msisdn:%s", msisdn)
}

// GetExistingMSISDNS fetches MSISDNS that already exist in the database from a given list.
func (repo *UserBaseRepository) GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	// Use the ANY operator with pq.Array to match multiple MSISDNS in a single query
	query := `
		SELECT msisdn
		FROM userbase
		WHERE msisdn = ANY($1)
	`
	rows, err := repo.db.QueryContext(ctx, query, pq.Array(msisdns))
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	// Collect existing MSISDNS from the result
	var existingMSISDNS []string
	for rows.Next() {
		var msisdn string
		if err := rows.Scan(&msisdn); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		existingMSISDNS = append(existingMSISDNS, msisdn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %v", err)
	}

	return existingMSISDNS, nil
}

// InsertUserRecords inserts a batch of UserBase records into the database
func (repo *UserBaseRepository) InsertUserRecords(ctx context.Context, records []*domain.UserBase) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO userbase (msisdn, type) VALUES ($1, $2) ON CONFLICT (msisdn) DO UPDATE SET type = EXCLUDED.type")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, record := range records {
		_, err = stmt.ExecContext(ctx, record.Msisdn, record.Type)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %v", err)
		}
	}

	return nil
}

// LoadExclusionList loads MSISDNS for Premier, Staff, and Blacklisted users in memory for batch filtering
func (repo *UserBaseRepository) LoadExclusionList() (map[string]bool, error) {
	cacheKey := "__USER_BASE_LIST__"
	cachedData, err := repo.redis.Get(repo.ctx, cacheKey)
	if err == nil {
		var listResponse map[string]bool
		if err := json.Unmarshal([]byte(cachedData), &listResponse); err == nil {
			return listResponse, nil
		}
	} else {
		repo.logger.Info("Failed to find cached data: ", zap.Error(err))
	}

	rows, err := repo.db.Query("SELECT msisdn FROM userbase WHERE type IN ('Premier', 'Staff', 'BLACKLISTED')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msisdnSet := make(map[string]bool)
	for rows.Next() {
		var msisdn string
		if err := rows.Scan(&msisdn); err != nil {
			return nil, err
		}
		msisdnSet[msisdn] = true
	}

	data, err := json.Marshal(msisdnSet)
	if err == nil {
		_ = repo.redis.Set(repo.ctx, cacheKey, data, 30*time.Minute)
	}
	return msisdnSet, nil
}

// IsExcludedUser checks if the UserIdentifier exists and has a Premier, Staff, or Blacklisted user type
// It first checks the exclusion list, then Redis cache, and finally falls back to the database if needed
func (repo *UserBaseRepository) IsExcludedUser(msisdn string) (bool, error) {
	// Attempt to load the exclusion list
	exclusionList, err := repo.LoadExclusionList()
	if err != nil {
		repo.logger.Warn("Failed to load exclusion list, falling back to Redis and DB", zap.Error(err))
	}

	// Check if UserIdentifier is in the exclusion list
	if exclusionList != nil {
		if _, exists := exclusionList[msisdn]; exists {
			return true, nil
		}
	}

	// Fallback to Redis cache
	cachedType, err := repo.redis.Get(repo.ctx, cacheKey(msisdn))
	if errors.Is(err, redis.Nil) {
		// Cache miss, check database
		var userType string
		query := `SELECT type FROM userbase WHERE msisdn = $1 AND type IN ('Premier', 'Staff', 'BLACKLISTED') LIMIT 1`
		err := repo.db.QueryRow(query, msisdn).Scan(&userType)
		if errors.Is(err, sql.ErrNoRows) {
			// UserIdentifier is not an excluded user, cache as Non-Excluded
			_ = repo.redis.Set(repo.ctx, cacheKey(msisdn), "Non-Excluded", time.Hour*24)
			return false, nil
		} else if err != nil {
			return false, fmt.Errorf("database error: %v", err)
		}

		// Cache the result as excluded user type
		_ = repo.redis.Set(repo.ctx, cacheKey(msisdn), userType, time.Hour*24)
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("redis error: %v", err)
	}

	// Return true if cached type is "Premier", "Staff", or "BLACKLISTED"
	return cachedType == "Premier" || cachedType == "Staff" || cachedType == "BLACKLISTED", nil
}

// FilterMSISDNS filters out MSISDNS that belong to Premier, Staff, or Blacklisted users
// It leverages the exclusion list, Redis, and falls back to the database if necessary
func (repo *UserBaseRepository) FilterMSISDNS(msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	var validMSISDNS []string

	// Load exclusion list (may be nil on error)
	exclusionList, err := repo.LoadExclusionList()
	if err != nil {
		repo.logger.Warn("Failed to load exclusion list, falling back to Redis and DB", zap.Error(err))
	}

	// First, exclude any MSISDNs explicitly present in exclusion list
	toCheck := make([]string, 0, len(msisdns))
	for _, msisdn := range msisdns {
		if exclusionList != nil {
			if _, ok := exclusionList[msisdn]; ok {
				// Skip Premier/Staff from exclusion list
				repo.logger.Debug("MSISDN excluded by exclusion list", zap.String("msisdn", msisdn))
				continue
			}
		}
		toCheck = append(toCheck, msisdn)
	}

	if len(toCheck) == 0 {
		return validMSISDNS, nil
	}

	// Redis lookup for remaining
	ctx := context.Background()
	keys := make([]string, len(toCheck))
	for i, msisdn := range toCheck {
		keys[i] = cacheKey(msisdn)
	}

	cachedResults, err := repo.redis.MGet(ctx, keys...)
	if err != nil {
		return nil, fmt.Errorf("redis error: %v", err)
	}

	// Determine which need DB lookup
	toDB := make([]string, 0, len(toCheck))
	cacheHits := 0
	cacheValidHits := 0
	for i, result := range cachedResults {
		if result == nil {
			// Cache miss
			toDB = append(toDB, toCheck[i])
			continue
		}
		cacheHits++
		if s, ok := result.(string); ok {
			switch s {
			case "Non-Premier/Staff":
				validMSISDNS = append(validMSISDNS, toCheck[i])
				cacheValidHits++
				continue
			case "Premier", "Staff", "Premier/Staff":
				// If exclusion list confirms, exclude; otherwise revalidate with DB to correct stale/mis-cached entries
				if exclusionList != nil {
					if _, exists := exclusionList[toCheck[i]]; exists {
						// confirmed exclude
						continue
					}
					// Not in exclusion list; verify with DB
					toDB = append(toDB, toCheck[i])
					continue
				}
				// No exclusion list available; trust cache and exclude
				continue
			default:
				// Unknown marker, send to DB for verification
				repo.logger.Debug("Unknown cache value, sending to DB", zap.String("msisdn", toCheck[i]), zap.String("value", s))
				toDB = append(toDB, toCheck[i])
			}
		} else {
			// Unexpected type from Redis, verify with DB
			repo.logger.Debug("Unexpected Redis type, sending to DB", zap.String("msisdn", toCheck[i]), zap.Any("value", result))
			toDB = append(toDB, toCheck[i])
		}
	}

	// Database fallback for uncached/unknown MSISDNS
	if len(toDB) > 0 {
		query := `
			SELECT msisdn 
			FROM userbase 
			WHERE msisdn = ANY($1) AND type IN ('Premier', 'Staff', 'BLACKLISTED')
		`
		rows, err := repo.db.Query(query, pq.Array(toDB))
		if err != nil {
			return nil, fmt.Errorf("database error: %v", err)
		}
		defer rows.Close()

		// Track found premier/staff to exclude and cache; remainder are valid
		foundPremier := make(map[string]bool, len(toDB))

		// Prepare for batch caching with Redis pipeline
		dbExcludedCount := 0
		for rows.Next() {
			var msisdn string
			if err := rows.Scan(&msisdn); err != nil {
				return nil, fmt.Errorf("error scanning row: %v", err)
			}
			foundPremier[msisdn] = true
			_ = repo.redis.Set(ctx, cacheKey(msisdn), "Premier/Staff", 24*time.Hour)
			dbExcludedCount++
		}

		// For the remainder (not premier/staff), cache as Non-Premier/Staff and include
		dbValidCount := 0
		for _, msisdn := range toDB {
			if !foundPremier[msisdn] {
				validMSISDNS = append(validMSISDNS, msisdn)
				_ = repo.redis.Set(ctx, cacheKey(msisdn), "Non-Premier/Staff", 24*time.Hour)
				dbValidCount++
			}
		}
	}

	return validMSISDNS, nil
}

// GetInvalidMSISDNS fetches MSISDNs that exist in the invalid_msisdn_logs table from a given list.
func (repo *UserBaseRepository) GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	// Use the ANY operator with pq.Array to match multiple MSISDNS in a single query
	query := `
		SELECT DISTINCT msisdn
		FROM invalid_msisdn_logs
		WHERE msisdn = ANY($1)
	`
	rows, err := repo.db.QueryContext(ctx, query, pq.Array(msisdns))
	if err != nil {
		return nil, fmt.Errorf("database query error for invalid MSISDNS: %v", err)
	}
	defer rows.Close()

	// Collect invalid MSISDNS from the result
	var invalidMSISDNS []string
	for rows.Next() {
		var msisdn string
		if err := rows.Scan(&msisdn); err != nil {
			return nil, fmt.Errorf("error scanning invalid MSISDN row: %v", err)
		}
		invalidMSISDNS = append(invalidMSISDNS, msisdn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading invalid MSISDN rows: %v", err)
	}

	return invalidMSISDNS, nil
}

// GetInvalidMSISDNSOptimized is an optimized version that uses EXISTS for better performance
// and implements caching to reduce database calls
func (repo *UserBaseRepository) GetInvalidMSISDNSOptimized(ctx context.Context, msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	// Check cache first for frequently accessed MSISDNs
	cachedInvalid, uncached := repo.getCachedInvalidMSISDNS(ctx, msisdns)

	// If all MSISDNs were cached, return immediately
	if len(uncached) == 0 {
		return cachedInvalid, nil
	}

	// Use EXISTS for better performance on large datasets
	query := `
		SELECT m.msisdn
		FROM unnest($1::text[]) AS m(msisdn)
		WHERE EXISTS (
			SELECT 1 
			FROM invalid_msisdn_logs 
			WHERE invalid_msisdn_logs.msisdn = m.msisdn
			LIMIT 1
		)
	`

	rows, err := repo.db.QueryContext(ctx, query, pq.Array(uncached))
	if err != nil {
		return nil, fmt.Errorf("database query error for invalid MSISDNS: %v", err)
	}
	defer rows.Close()

	// Collect invalid MSISDNS from the result
	var dbInvalidMSISDNS []string
	for rows.Next() {
		var msisdn string
		if err := rows.Scan(&msisdn); err != nil {
			return nil, fmt.Errorf("error scanning invalid MSISDN row: %v", err)
		}
		dbInvalidMSISDNS = append(dbInvalidMSISDNS, msisdn)

		// Cache the result for future use
		repo.cacheInvalidMSISDN(ctx, msisdn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading invalid MSISDN rows: %v", err)
	}

	// Combine cached and database results
	result := append(cachedInvalid, dbInvalidMSISDNS...)
	return result, nil
}

// GetInvalidMSISDNSFast is the fastest version for single MSISDN lookups
// Uses direct cache check and minimal database query
func (repo *UserBaseRepository) GetInvalidMSISDNSFast(ctx context.Context, msisdn string) (bool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("invalid_msisdn:%s", msisdn)
	exists, err := repo.redis.Exists(ctx, cacheKey)
	if err == nil && exists > 0 {
		return true, nil // Found in cache - it's invalid
	}

	// Not in cache, check database with optimized query
	query := `
		SELECT 1 
		FROM invalid_msisdn_logs 
		WHERE msisdn = $1 
		LIMIT 1
	`

	var result int
	err = repo.db.QueryRowContext(ctx, query, msisdn).Scan(&result)
	if err == sql.ErrNoRows {
		return false, nil // Not found - it's valid
	}
	if err != nil {
		return false, fmt.Errorf("database query error: %v", err)
	}

	// Cache the result for future use
	repo.cacheInvalidMSISDN(ctx, msisdn)
	return true, nil
}

// getCachedInvalidMSISDNS checks Redis cache for invalid MSISDNs and returns cached results
// along with a list of uncached MSISDNs that need database lookup
func (repo *UserBaseRepository) getCachedInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, []string) {
	if len(msisdns) == 0 {
		return []string{}, []string{}
	}

	// Prepare cache keys
	cacheKeys := make([]string, len(msisdns))
	for i, msisdn := range msisdns {
		cacheKeys[i] = fmt.Sprintf("invalid_msisdn:%s", msisdn)
	}

	// Batch check cache
	cachedResults, err := repo.redis.MGet(ctx, cacheKeys...)
	if err != nil {
		repo.logger.Warn("Failed to check cache, falling back to database", zap.Error(err))
		return []string{}, msisdns
	}

	var cachedInvalid []string
	var uncached []string

	for i, result := range cachedResults {
		if result != nil {
			// Found in cache
			cachedInvalid = append(cachedInvalid, msisdns[i])
		} else {
			// Not in cache, needs database lookup
			uncached = append(uncached, msisdns[i])
		}
	}

	return cachedInvalid, uncached
}

// cacheInvalidMSISDN caches an invalid MSISDN in Redis with TTL
func (repo *UserBaseRepository) cacheInvalidMSISDN(ctx context.Context, msisdn string) {
	cacheKey := fmt.Sprintf("invalid_msisdn:%s", msisdn)
	// Cache for 24 hours - invalid MSISDNs don't change frequently
	err := repo.redis.Set(ctx, cacheKey, "1", 24*time.Hour)
	if err != nil {
		repo.logger.Warn("Failed to cache invalid MSISDN",
			zap.String("msisdn", msisdn),
			zap.Error(err))
	}
}

// GetInvalidMSISDNSStats returns statistics about the invalid_msisdn_logs table
func (repo *UserBaseRepository) GetInvalidMSISDNSStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_records,
			COUNT(DISTINCT msisdn) as unique_msisdns,
			MIN(created_at) as oldest_record,
			MAX(created_at) as newest_record,
			pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) as table_size
		FROM invalid_msisdn_logs
	`

	var totalRecords, uniqueMSISDNS int
	var oldestRecord, newestRecord time.Time
	var tableSize string

	err := repo.db.QueryRowContext(ctx, query).Scan(
		&totalRecords, &uniqueMSISDNS, &oldestRecord, &newestRecord, &tableSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %v", err)
	}

	return map[string]interface{}{
		"total_records":  totalRecords,
		"unique_msisdns": uniqueMSISDNS,
		"oldest_record":  oldestRecord,
		"newest_record":  newestRecord,
		"table_size":     tableSize,
	}, nil
}

// GetBlacklistedMSISDNS fetches MSISDNs that are blacklisted in the userbase table from a given list.
func (repo *UserBaseRepository) GetBlacklistedMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	// Use the ANY operator with pq.Array to match multiple MSISDNS in a single query
	query := `
		SELECT DISTINCT msisdn
		FROM userbase
		WHERE msisdn = ANY($1) AND type = 'BLACKLISTED'
	`
	rows, err := repo.db.QueryContext(ctx, query, pq.Array(msisdns))
	if err != nil {
		return nil, fmt.Errorf("database query error for blacklisted MSISDNS: %v", err)
	}
	defer rows.Close()

	// Collect blacklisted MSISDNS from the result
	var blacklistedMSISDNS []string
	for rows.Next() {
		var msisdn string
		if err := rows.Scan(&msisdn); err != nil {
			return nil, fmt.Errorf("error scanning blacklisted MSISDN row: %v", err)
		}
		blacklistedMSISDNS = append(blacklistedMSISDNS, msisdn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading blacklisted MSISDN rows: %v", err)
	}

	return blacklistedMSISDNS, nil
}
