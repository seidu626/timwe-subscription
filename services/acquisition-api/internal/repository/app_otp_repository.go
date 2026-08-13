package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// AppOTPRepository handles Dayline app_login_otps data access.
type AppOTPRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewAppOTPRepository creates a new AppOTPRepository.
func NewAppOTPRepository(db *sql.DB, logger *zap.Logger) *AppOTPRepository {
	return &AppOTPRepository{db: db, logger: logger}
}

// Create inserts a new OTP row and returns its assigned ID.
func (r *AppOTPRepository) Create(otp *domain.AppLoginOTP) (int64, error) {
	var id int64
	err := r.db.QueryRow(`
		INSERT INTO app_login_otps (msisdn, tenant_key, code_hash, code_salt, expires_at, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, 0, $6)
		RETURNING id
	`, otp.MSISDN, otp.TenantKey, otp.CodeHash, otp.CodeSalt, otp.ExpiresAt, otp.CreatedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create app login otp: %w", err)
	}
	return id, nil
}

// CountActiveSince counts OTP requests for msisdn/tenant created at or after
// since, used for the "max 3 active requests/msisdn/hour" rate limit.
func (r *AppOTPRepository) CountActiveSince(msisdn, tenantKey string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM app_login_otps
		WHERE msisdn = $1 AND tenant_key = $2 AND created_at >= $3
	`, msisdn, tenantKey, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active app login otps: %w", err)
	}
	return count, nil
}

// FindLatestUnconsumed returns the most recent OTP row for msisdn/tenant that
// has not yet been consumed (regardless of expiry, so callers can distinguish
// OTP_EXPIRED from OTP_INVALID/NOT_FOUND).
func (r *AppOTPRepository) FindLatestUnconsumed(msisdn, tenantKey string) (*domain.AppLoginOTP, error) {
	var otp domain.AppLoginOTP
	var consumedAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT id, msisdn, tenant_key, code_hash, code_salt, expires_at, attempts, consumed_at, created_at
		FROM app_login_otps
		WHERE msisdn = $1 AND tenant_key = $2 AND consumed_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, msisdn, tenantKey).Scan(&otp.ID, &otp.MSISDN, &otp.TenantKey, &otp.CodeHash, &otp.CodeSalt,
		&otp.ExpiresAt, &otp.Attempts, &consumedAt, &otp.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find app login otp: %w", err)
	}
	if consumedAt.Valid {
		otp.ConsumedAt = &consumedAt.Time
	}
	return &otp, nil
}

// IncrementAttempts atomically increments the attempt counter and returns the
// new count, so the caller can compare against AppLoginOTPMaxAttempts without
// a separate read-modify-write race.
func (r *AppOTPRepository) IncrementAttempts(id int64) (int, error) {
	var attempts int
	err := r.db.QueryRow(`
		UPDATE app_login_otps SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts
	`, id).Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("failed to increment app login otp attempts: %w", err)
	}
	return attempts, nil
}

// MarkConsumed marks an OTP row as consumed (verified or invalidated), so it
// can never be reused.
func (r *AppOTPRepository) MarkConsumed(id int64, now time.Time) error {
	_, err := r.db.Exec(`UPDATE app_login_otps SET consumed_at = $2 WHERE id = $1`, id, now)
	if err != nil {
		return fmt.Errorf("failed to mark app login otp consumed: %w", err)
	}
	return nil
}
