package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

// appMSISDNRe validates E.164-without-plus MSISDNs, e.g. "233241234567",
// per the app API contract's MSISDN format convention.
var appMSISDNRe = regexp.MustCompile(`^[1-9]\d{7,14}$`)

// ValidateAppMSISDN reports whether msisdn matches the contract's E.164
// (without leading '+') format.
func ValidateAppMSISDN(msisdn string) bool {
	return appMSISDNRe.MatchString(strings.TrimSpace(msisdn))
}

// OTPSender delivers a login OTP code by SMS. There is no production
// implementation of this interface wired in cmd/main.go: acquisition-api has
// no callable outbound SMS ingress (message_outbox requires an existing
// subscriptions row; the notification service's HTTP surface is inbound
// webhook receivers only). See the result capsule's blocked_on entry.
type OTPSender interface {
	SendLoginOTP(msisdn, tenantKey, code string) error
}

// AppOTPService implements the Dayline app login OTP lifecycle:
// request (generate+persist+send), verify (check+consume), with TTL,
// attempt-exhaustion, and per-msisdn rate limiting per the app API contract.
type AppOTPService struct {
	repo   *repository.AppOTPRepository
	sender OTPSender
	clock  func() time.Time
	logger *zap.Logger
}

// NewAppOTPService creates a new AppOTPService. sender may be nil; see OTPSender.
func NewAppOTPService(repo *repository.AppOTPRepository, sender OTPSender, logger *zap.Logger) *AppOTPService {
	return &AppOTPService{repo: repo, sender: sender, clock: time.Now, logger: logger}
}

// SetClock overrides the service's time source (tests only).
func (s *AppOTPService) SetClock(clock func() time.Time) {
	if clock != nil {
		s.clock = clock
	}
}

// RequestOTP validates msisdn, enforces the active-request rate limit,
// generates and persists a new OTP, and dispatches it via OTPSender.
func (s *AppOTPService) RequestOTP(msisdn, tenantKey string) error {
	msisdn = strings.TrimSpace(msisdn)
	tenantKey = strings.TrimSpace(tenantKey)
	if !ValidateAppMSISDN(msisdn) {
		return domain.NewAppError(domain.AppErrInvalidMSISDN, "msisdn must be E.164 without a leading plus")
	}
	if tenantKey == "" {
		return domain.NewAppError(domain.AppErrValidation, "tenant is required")
	}

	now := s.clock()
	activeCount, err := s.repo.CountActiveSince(msisdn, tenantKey, now.Add(-domain.AppLoginOTPRateLimitWindow))
	if err != nil {
		return err
	}
	if activeCount >= domain.AppLoginOTPMaxActivePerHr {
		return domain.NewAppError(domain.AppErrRateLimited, "too many otp requests, try again later")
	}

	code, err := generateOTPCode(domain.AppLoginOTPCodeLength)
	if err != nil {
		return fmt.Errorf("failed to generate otp code: %w", err)
	}
	salt, err := generateOTPSalt()
	if err != nil {
		return fmt.Errorf("failed to generate otp salt: %w", err)
	}

	otp := &domain.AppLoginOTP{
		MSISDN:    msisdn,
		TenantKey: tenantKey,
		CodeHash:  hashOTPCode(salt, code),
		CodeSalt:  salt,
		ExpiresAt: now.Add(domain.AppLoginOTPTTL),
		CreatedAt: now,
	}
	if _, err := s.repo.Create(otp); err != nil {
		return err
	}

	if s.sender == nil {
		s.logger.Error("app login otp sms sender not configured; otp persisted but not delivered",
			zap.String("tenant_key", tenantKey))
		return domain.NewAppError(domain.AppErrProviderError, "otp delivery is currently unavailable")
	}
	if err := s.sender.SendLoginOTP(msisdn, tenantKey, code); err != nil {
		s.logger.Error("failed to send app login otp", zap.Error(err), zap.String("tenant_key", tenantKey))
		return domain.NewAppError(domain.AppErrProviderError, "failed to send otp")
	}
	return nil
}

// VerifyOTP checks code against the latest unconsumed OTP for msisdn/tenant,
// enforcing expiry and the max-attempts limit, and consumes the OTP row on
// any terminal outcome (success, expiry, or attempt exhaustion) so it cannot
// be reused.
func (s *AppOTPService) VerifyOTP(msisdn, tenantKey, code string) error {
	msisdn = strings.TrimSpace(msisdn)
	tenantKey = strings.TrimSpace(tenantKey)
	code = strings.TrimSpace(code)
	if !ValidateAppMSISDN(msisdn) {
		return domain.NewAppError(domain.AppErrInvalidMSISDN, "msisdn must be E.164 without a leading plus")
	}
	if code == "" {
		return domain.NewAppError(domain.AppErrOTPInvalid, "code is required")
	}

	otp, err := s.repo.FindLatestUnconsumed(msisdn, tenantKey)
	if err != nil {
		return err
	}
	if otp == nil {
		return domain.NewAppError(domain.AppErrOTPInvalid, "no active otp for this msisdn")
	}

	now := s.clock()
	if now.After(otp.ExpiresAt) {
		_ = s.repo.MarkConsumed(otp.ID, now)
		return domain.NewAppError(domain.AppErrOTPExpired, "otp has expired")
	}
	if otp.Attempts >= domain.AppLoginOTPMaxAttempts {
		_ = s.repo.MarkConsumed(otp.ID, now)
		return domain.NewAppError(domain.AppErrOTPInvalid, "otp has too many failed attempts")
	}

	expected := hashOTPCode(otp.CodeSalt, code)
	if !hmac.Equal([]byte(expected), []byte(otp.CodeHash)) {
		attempts, incErr := s.repo.IncrementAttempts(otp.ID)
		if incErr == nil && attempts >= domain.AppLoginOTPMaxAttempts {
			_ = s.repo.MarkConsumed(otp.ID, now)
		}
		return domain.NewAppError(domain.AppErrOTPInvalid, "invalid otp code")
	}

	if err := s.repo.MarkConsumed(otp.ID, now); err != nil {
		return err
	}
	return nil
}

func generateOTPCode(length int) (string, error) {
	max := 1
	for i := 0; i < length; i++ {
		max *= 10
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := (int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])) & 0x7fffffff
	n %= max
	return fmt.Sprintf("%0*d", length, n), nil
}

func generateOTPSalt() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashOTPCode(salt, code string) string {
	sum := sha256.Sum256([]byte(salt + code))
	return hex.EncodeToString(sum[:])
}
