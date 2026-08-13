package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// OTPSender delivers a login OTP code by SMS. The production implementation
// is TenantSMSSender, which posts through the SMS gateway configured per
// tenant in tenant_channel_credentials (purpose sms_api).
type OTPSender interface {
	SendLoginOTP(msisdn, tenantKey, code string) error
}

// DelegatedOTPProvider is an external service that owns OTP code generation,
// delivery and verification for tenants configured to use it. The production
// implementation is ArkeselOTPProvider.
//
// Delegation covers code custody only. This service keeps the request rate
// limit and the per-OTP attempt ceiling in front of every Verify call, because
// providers do not necessarily enforce one: Arkesel's verify endpoint accepts
// unlimited wrong guesses against a live code, which without a local ceiling
// would make a 6-digit code brute-forceable within its own lifetime.
type DelegatedOTPProvider interface {
	// Configured reports whether tenantKey delegates its OTP lifecycle.
	Configured(ctx context.Context, tenantKey string) (bool, error)
	// Generate asks the provider to mint and deliver a code.
	Generate(ctx context.Context, msisdn, tenantKey string) error
	// Verify submits a user-supplied code, returning
	// ErrDelegatedOTPCodeInvalid or ErrDelegatedOTPExpired for those outcomes.
	Verify(ctx context.Context, msisdn, tenantKey, code string) error
}

// AppOTPService implements the Dayline app login OTP lifecycle:
// request (generate+persist+send), verify (check+consume), with TTL,
// attempt-exhaustion, and per-msisdn rate limiting per the app API contract.
type AppOTPService struct {
	repo   *repository.AppOTPRepository
	sender OTPSender
	clock  func() time.Time
	logger *zap.Logger

	// delegate is optional. When set, tenants holding an ACTIVE otp_api
	// credential delegate code custody to it; every other tenant is unaffected.
	delegate DelegatedOTPProvider
}

// NewAppOTPService creates a new AppOTPService. sender may be nil; see OTPSender.
func NewAppOTPService(repo *repository.AppOTPRepository, sender OTPSender, logger *zap.Logger) *AppOTPService {
	return &AppOTPService{repo: repo, sender: sender, clock: time.Now, logger: logger}
}

// SetDelegatedProvider enables delegated OTP for tenants that have an ACTIVE
// otp_api credential. Leaving it unset keeps every tenant on the local
// lifecycle, so the feature is off until an operator both wires the provider
// and binds a credential.
func (s *AppOTPService) SetDelegatedProvider(delegate DelegatedOTPProvider) {
	s.delegate = delegate
}

// delegatedFor reports whether tenantKey's OTP lifecycle is delegated. A
// resolution error is propagated rather than treated as "not delegated": a
// broken credential must fail the request, not silently switch auth paths.
func (s *AppOTPService) delegatedFor(tenantKey string) (bool, error) {
	if s.delegate == nil {
		return false, nil
	}
	ctx, cancel := otpProviderContext()
	defer cancel()
	return s.delegate.Configured(ctx, tenantKey)
}

// otpProviderContext bounds delegated-provider calls; the service API takes no
// context of its own, matching how TenantSMSSender bounds gateway calls.
func otpProviderContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
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

	delegated, err := s.delegatedFor(tenantKey)
	if err != nil {
		s.logger.Error("failed to resolve delegated otp configuration",
			zap.Error(err), zap.String("tenant_key", tenantKey))
		return domain.NewAppError(domain.AppErrProviderError, "otp delivery is currently unavailable")
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

	if delegated {
		// In delegated mode the row exists purely for the rate limit and the
		// attempt ceiling: the provider mints and holds the real code. The
		// hash stored above is of a code that is discarded and never sent, so
		// if the tenant is switched back to the local lifecycle while this OTP
		// is still live, local verification fails closed instead of accepting
		// a guess.
		ctx, cancel := otpProviderContext()
		defer cancel()
		if err := s.delegate.Generate(ctx, msisdn, tenantKey); err != nil {
			s.logger.Error("failed to generate delegated app login otp",
				zap.Error(err), zap.String("tenant_key", tenantKey))
			return domain.NewAppError(domain.AppErrProviderError, "failed to send otp")
		}
		return nil
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

	delegated, err := s.delegatedFor(tenantKey)
	if err != nil {
		s.logger.Error("failed to resolve delegated otp configuration",
			zap.Error(err), zap.String("tenant_key", tenantKey))
		return domain.NewAppError(domain.AppErrProviderError, "otp verification is currently unavailable")
	}

	if delegated {
		// The expiry and attempt checks above already ran, so the provider,
		// which enforces no ceiling of its own, is only ever reached while
		// this OTP still has attempts left.
		ctx, cancel := otpProviderContext()
		defer cancel()
		switch err := s.delegate.Verify(ctx, msisdn, tenantKey, code); {
		case err == nil:
		case errors.Is(err, ErrDelegatedOTPExpired):
			_ = s.repo.MarkConsumed(otp.ID, now)
			return domain.NewAppError(domain.AppErrOTPExpired, "otp has expired")
		case errors.Is(err, ErrDelegatedOTPCodeInvalid):
			return s.recordFailedAttempt(otp.ID, now)
		default:
			s.logger.Error("delegated app login otp verification failed",
				zap.Error(err), zap.String("tenant_key", tenantKey))
			return domain.NewAppError(domain.AppErrProviderError, "otp verification is currently unavailable")
		}
	} else {
		expected := hashOTPCode(otp.CodeSalt, code)
		if !hmac.Equal([]byte(expected), []byte(otp.CodeHash)) {
			return s.recordFailedAttempt(otp.ID, now)
		}
	}

	if err := s.repo.MarkConsumed(otp.ID, now); err != nil {
		return err
	}
	return nil
}

// recordFailedAttempt increments the OTP's attempt counter, consuming the row
// once the ceiling is reached, and returns the caller's invalid-code error.
// This ceiling is the only brute-force protection in delegated mode.
func (s *AppOTPService) recordFailedAttempt(otpID int64, now time.Time) error {
	attempts, incErr := s.repo.IncrementAttempts(otpID)
	if incErr == nil && attempts >= domain.AppLoginOTPMaxAttempts {
		_ = s.repo.MarkConsumed(otpID, now)
	}
	return domain.NewAppError(domain.AppErrOTPInvalid, "invalid otp code")
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
