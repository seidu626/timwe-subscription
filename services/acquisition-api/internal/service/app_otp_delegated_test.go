package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
)

// fakeDelegate records what the service asked of a delegated OTP provider.
type fakeDelegate struct {
	configured    bool
	configuredErr error
	generateErr   error
	verifyErr     error

	generateCalls int
	verifyCalls   int
	lastCode      string
}

func (f *fakeDelegate) Configured(ctx context.Context, tenantKey string) (bool, error) {
	return f.configured, f.configuredErr
}

func (f *fakeDelegate) Generate(ctx context.Context, msisdn, tenantKey string) error {
	f.generateCalls++
	return f.generateErr
}

func (f *fakeDelegate) Verify(ctx context.Context, msisdn, tenantKey, code string) error {
	f.verifyCalls++
	f.lastCode = code
	return f.verifyErr
}

func newDelegatedOTPTestService(t *testing.T, delegate DelegatedOTPProvider, sender OTPSender) (*AppOTPService, sqlmock.Sqlmock, time.Time) {
	t.Helper()
	svc, mock := newAppOTPTestService(t, sender)
	svc.SetDelegatedProvider(delegate)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })
	return svc, mock, now
}

func TestRequestOTP_DelegatedGeneratesAtProviderAndSkipsSMSSender(t *testing.T) {
	delegate := &fakeDelegate{configured: true}
	sender := &fakeOTPSender{}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, sender)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "careerify", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO app_login_otps`).
		WithArgs("233241234567", "careerify", sqlmock.AnyArg(), sqlmock.AnyArg(), now.Add(domain.AppLoginOTPTTL), now).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	if err := svc.RequestOTP("233241234567", "careerify"); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if delegate.generateCalls != 1 {
		t.Errorf("delegate.Generate calls = %d, want 1", delegate.generateCalls)
	}
	// The provider delivers the message, so sending our own would mean two
	// codes in flight for one login.
	if sender.sent {
		t.Error("sms sender must not run in delegated mode")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_DelegatedRateLimitStillApplies(t *testing.T) {
	delegate := &fakeDelegate{configured: true}
	svc, mock, _ := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "careerify", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(domain.AppLoginOTPMaxActivePerHr))

	err := svc.RequestOTP("233241234567", "careerify")
	requireAppError(t, err, domain.AppErrRateLimited)
	if delegate.generateCalls != 0 {
		t.Errorf("rate-limited request reached the provider %d times", delegate.generateCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_DelegateResolutionFailureFailsClosed(t *testing.T) {
	delegate := &fakeDelegate{configuredErr: context.DeadlineExceeded}
	sender := &fakeOTPSender{}
	svc, mock, _ := newDelegatedOTPTestService(t, delegate, sender)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "careerify", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err := svc.RequestOTP("233241234567", "careerify")
	requireAppError(t, err, domain.AppErrProviderError)
	// An unresolvable credential must not quietly fall back to the local
	// lifecycle, which is a different authentication path.
	if sender.sent || delegate.generateCalls != 0 {
		t.Error("failed resolution must not deliver an otp by either path")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRequestOTP_UnconfiguredTenantKeepsLocalLifecycle(t *testing.T) {
	delegate := &fakeDelegate{configured: false}
	sender := &fakeOTPSender{}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, sender)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO app_login_otps`).
		WithArgs("233241234567", "nrg", sqlmock.AnyArg(), sqlmock.AnyArg(), now.Add(domain.AppLoginOTPTTL), now).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	if err := svc.RequestOTP("233241234567", "nrg"); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if !sender.sent || !appOTPCodePattern.MatchString(sender.code) {
		t.Errorf("local tenant should still get a locally generated code, got sent=%v code=%q", sender.sent, sender.code)
	}
	if delegate.generateCalls != 0 {
		t.Error("unconfigured tenant must not reach the provider")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_DelegatedSuccessConsumesRow(t *testing.T) {
	delegate := &fakeDelegate{configured: true}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "careerify").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "careerify", "unused-hash", "unused-salt", now.Add(time.Minute), 0, nil, now))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.VerifyOTP("233241234567", "careerify", "482913"); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	if delegate.verifyCalls != 1 || delegate.lastCode != "482913" {
		t.Errorf("delegate saw %d calls, last code %q", delegate.verifyCalls, delegate.lastCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// The provider accepts unlimited wrong guesses against a live code, so this
// local ceiling is the only thing standing between a 6-digit code and a brute
// force. Losing it would not fail any other test.
func TestVerifyOTP_DelegatedWrongCodeConsumesRowAtAttemptCeiling(t *testing.T) {
	delegate := &fakeDelegate{configured: true, verifyErr: ErrDelegatedOTPCodeInvalid}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "careerify").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "careerify", "unused-hash", "unused-salt",
				now.Add(time.Minute), domain.AppLoginOTPMaxAttempts-1, nil, now))
	mock.ExpectQuery(`UPDATE app_login_otps SET attempts = attempts \+ 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"attempts"}).AddRow(domain.AppLoginOTPMaxAttempts))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.VerifyOTP("233241234567", "careerify", "000000")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_DelegatedExhaustedAttemptsNeverReachProvider(t *testing.T) {
	delegate := &fakeDelegate{configured: true, verifyErr: ErrDelegatedOTPCodeInvalid}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "careerify").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "careerify", "unused-hash", "unused-salt",
				now.Add(time.Minute), domain.AppLoginOTPMaxAttempts, nil, now))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.VerifyOTP("233241234567", "careerify", "000000")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if delegate.verifyCalls != 0 {
		t.Errorf("exhausted otp still reached the provider %d times", delegate.verifyCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_DelegatedExpirySentinelMapsToExpiredCode(t *testing.T) {
	delegate := &fakeDelegate{configured: true, verifyErr: ErrDelegatedOTPExpired}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "careerify").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "careerify", "unused-hash", "unused-salt", now.Add(time.Minute), 0, nil, now))
	mock.ExpectExec(`UPDATE app_login_otps SET consumed_at`).
		WithArgs(int64(1), now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.VerifyOTP("233241234567", "careerify", "482913")
	requireAppError(t, err, domain.AppErrOTPExpired)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestVerifyOTP_DelegatedProviderOutageIsNotAVerdictOnTheCode(t *testing.T) {
	delegate := &fakeDelegate{configured: true, verifyErr: context.DeadlineExceeded}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "careerify").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "careerify", "unused-hash", "unused-salt", now.Add(time.Minute), 0, nil, now))

	err := svc.VerifyOTP("233241234567", "careerify", "482913")
	requireAppError(t, err, domain.AppErrProviderError)
	// No attempt is burned and the row is not consumed: the user's code was
	// never actually judged.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// A delegated row stores the hash of a code that was generated, discarded and
// never sent. If the tenant is moved back to the local lifecycle while that
// OTP is live, verification must fail rather than accept anything.
func TestVerifyOTP_DelegatedRowFailsClosedUnderLocalVerification(t *testing.T) {
	delegate := &fakeDelegate{configured: false}
	svc, mock, now := newDelegatedOTPTestService(t, delegate, &fakeOTPSender{})

	salt := "deadbeefdeadbeef"
	discarded := hashAppOTPForTest(salt, "999999")
	mock.ExpectQuery(`FROM app_login_otps`).
		WithArgs("233241234567", "careerify").
		WillReturnRows(sqlmock.NewRows(appOTPFindColumns).
			AddRow(int64(1), "233241234567", "careerify", discarded, salt, now.Add(time.Minute), 0, nil, now))
	mock.ExpectQuery(`UPDATE app_login_otps SET attempts = attempts \+ 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"attempts"}).AddRow(1))

	err := svc.VerifyOTP("233241234567", "careerify", "482913")
	requireAppError(t, err, domain.AppErrOTPInvalid)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
