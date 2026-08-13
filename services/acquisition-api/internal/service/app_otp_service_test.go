package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

var appOTPCodePattern = regexp.MustCompile(`^\d{6}$`)

type fakeOTPSender struct {
	sent    bool
	msisdn  string
	tenant  string
	code    string
	sendErr error
}

func (f *fakeOTPSender) SendLoginOTP(msisdn, tenantKey, code string) error {
	f.sent = true
	f.msisdn = msisdn
	f.tenant = tenantKey
	f.code = code
	return f.sendErr
}

func newAppOTPTestService(t *testing.T, sender OTPSender) (*AppOTPService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := repository.NewAppOTPRepository(db, zap.NewNop())
	svc := NewAppOTPService(repo, sender, zap.NewNop())
	return svc, mock
}

func TestRequestOTP_RejectsInvalidMSISDN(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})
	err := svc.RequestOTP("not-a-number", "nrg")
	appErr := requireAppError(t, err, domain.AppErrInvalidMSISDN)
	_ = appErr
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no db calls, got: %v", err)
	}
}

func TestRequestOTP_RateLimitedAfterMaxActive(t *testing.T) {
	sender := &fakeOTPSender{}
	svc, mock := newAppOTPTestService(t, sender)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(domain.AppLoginOTPMaxActivePerHr))

	err := svc.RequestOTP("233241234567", "nrg")
	requireAppError(t, err, domain.AppErrRateLimited)
	if sender.sent {
		t.Fatalf("sender should not be invoked when rate-limited")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_SuccessPersistsAndSends(t *testing.T) {
	sender := &fakeOTPSender{}
	svc, mock := newAppOTPTestService(t, sender)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg(), sqlmock.AnyArg(), now.Add(domain.AppLoginOTPTTL), now).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	if err := svc.RequestOTP("233241234567", "nrg"); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if !sender.sent {
		t.Fatalf("expected sender to be invoked")
	}
	if sender.msisdn != "233241234567" || sender.tenant != "nrg" {
		t.Fatalf("sender received unexpected msisdn/tenant: %+v", sender)
	}
	if !appOTPCodePattern.MatchString(sender.code) {
		t.Fatalf("expected a 6-digit code, got %q", sender.code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_NoSenderConfiguredFailsClosed(t *testing.T) {
	svc, mock := newAppOTPTestService(t, nil)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg(), sqlmock.AnyArg(), now.Add(domain.AppLoginOTPTTL), now).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	err := svc.RequestOTP("233241234567", "nrg")
	requireAppError(t, err, domain.AppErrProviderError)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_SenderFailurePropagatesProviderError(t *testing.T) {
	sender := &fakeOTPSender{sendErr: errors.New("sms gateway down")}
	svc, mock := newAppOTPTestService(t, sender)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg(), sqlmock.AnyArg(), now.Add(domain.AppLoginOTPTTL), now).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	err := svc.RequestOTP("233241234567", "nrg")
	requireAppError(t, err, domain.AppErrProviderError)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

var appOTPFindColumns = []string{"id", "msisdn", "tenant_key", "code_hash", "code_salt", "expires_at", "attempts", "consumed_at", "created_at"}

func hashAppOTPForTest(salt, code string) string {
	sum := sha256.Sum256([]byte(salt + code))
	return hex.EncodeToString(sum[:])
}

func TestVerifyOTP_SuccessConsumesOTP(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	salt := "deadbeefdeadbeef"
	code := "123456"
	hash := hashAppOTPForTest(salt, code)

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "nrg").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "nrg", hash, salt, now.Add(time.Minute), 0, nil, now))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.VerifyOTP("233241234567", "nrg", code); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_ExpiredMarksConsumedAndReturnsExpiredCode(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	salt := "deadbeefdeadbeef"
	code := "123456"
	hash := hashAppOTPForTest(salt, code)

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "nrg").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "nrg", hash, salt, now.Add(-time.Minute), 0, nil, now.Add(-10*time.Minute)))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.VerifyOTP("233241234567", "nrg", code)
	requireAppError(t, err, domain.AppErrOTPExpired)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_WrongCodeIncrementsAttemptsWithoutConsuming(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	salt := "deadbeefdeadbeef"
	hash := hashAppOTPForTest(salt, "123456")

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "nrg").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "nrg", hash, salt, now.Add(time.Minute), 0, nil, now))
	mock.ExpectQuery(`UPDATE app_login_otps SET attempts = attempts \+ 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"attempts"}).AddRow(1))

	err := svc.VerifyOTP("233241234567", "nrg", "000000")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations (no consume expected below max attempts): %v", err)
	}
}

func TestVerifyOTP_AttemptExhaustionMarksConsumed(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	salt := "deadbeefdeadbeef"
	hash := hashAppOTPForTest(salt, "123456")

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "nrg").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "nrg", hash, salt, now.Add(time.Minute), domain.AppLoginOTPMaxAttempts-1, nil, now))
	mock.ExpectQuery(`UPDATE app_login_otps SET attempts = attempts \+ 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"attempts"}).AddRow(domain.AppLoginOTPMaxAttempts))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.VerifyOTP("233241234567", "nrg", "000000")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_AlreadyExhaustedRowRejectsWithoutRecheck(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	salt := "deadbeefdeadbeef"
	hash := hashAppOTPForTest(salt, "123456")

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "nrg").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "nrg", hash, salt, now.Add(time.Minute), domain.AppLoginOTPMaxAttempts, nil, now))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.VerifyOTP("233241234567", "nrg", "123456")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_NoActiveOTPReturnsInvalid(t *testing.T) {
	svc, mock := newAppOTPTestService(t, &fakeOTPSender{})

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "nrg").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns))

	err := svc.VerifyOTP("233241234567", "nrg", "123456")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func requireAppError(t *testing.T, err error, want domain.AppErrorCode) *domain.AppError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("expected *domain.AppError, got %T (%v)", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("expected error code %s, got %s (%s)", want, appErr.Code, appErr.Message)
	}
	return appErr
}
