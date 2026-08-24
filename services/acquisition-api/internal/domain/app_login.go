package domain

import "time"

// AppLoginOTP is a Dayline mobile app login OTP (app_login_otps table).
//
// This is a distinct credential from the TIMWE billing opt-in PIN
// (AcquisitionTransaction.TransactionAuthCode) and must never share storage
// or delivery copy with it.
type AppLoginOTP struct {
	ID         int64
	MSISDN     string
	TenantKey  string
	CodeHash   string
	CodeSalt   string
	ExpiresAt  time.Time
	Attempts   int
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// App login OTP tunables, per the Dayline app API contract.
const (
	AppLoginOTPTTL             = 5 * time.Minute
	AppLoginOTPMaxAttempts     = 5
	AppLoginOTPMaxActivePerHr  = 3
	AppLoginOTPRateLimitWindow = time.Hour
	AppLoginOTPCodeLength      = 6
)

// AppErrorCode is one of the contract's fixed error codes for /v1/app/* responses.
type AppErrorCode string

const (
	AppErrInvalidMSISDN AppErrorCode = "INVALID_MSISDN"
	AppErrOTPInvalid    AppErrorCode = "OTP_INVALID"
	AppErrOTPExpired    AppErrorCode = "OTP_EXPIRED"
	AppErrRateLimited   AppErrorCode = "RATE_LIMITED"
	AppErrUnauthorized  AppErrorCode = "UNAUTHORIZED"
	AppErrNotFound      AppErrorCode = "NOT_FOUND"
	AppErrConflict      AppErrorCode = "CONFLICT"
	AppErrProviderError AppErrorCode = "PROVIDER_ERROR"
	AppErrValidation    AppErrorCode = "VALIDATION"
)

// AppError is the error type returned by /v1/app/* service methods; handlers
// translate it directly into the contract's {"error":{"code","message"}} envelope.
type AppError struct {
	Code    AppErrorCode
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

// NewAppError builds an AppError with the given code and message.
func NewAppError(code AppErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// AppCatalogProduct is a single catalog entry for GET /v1/app/catalog.
type AppCatalogProduct struct {
	Slug            string   `json:"slug"`
	Tenant          string   `json:"tenant"`
	TenantName      string   `json:"tenant_name"`
	Name            string   `json:"name"`
	Tagline         string   `json:"tagline,omitempty"`
	Description     string   `json:"description,omitempty"`
	Category        string   `json:"category,omitempty"`
	ArtworkURL      string   `json:"artwork_url,omitempty"`
	SampleContent   string   `json:"sample_content,omitempty"`
	Price           *float64 `json:"price,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	BillingCycle    string   `json:"billing_cycle,omitempty"`
	FlowType        FlowType `json:"flow_type"`
	SubscriberCount int      `json:"subscriber_count"`
	Featured        bool     `json:"featured,omitempty"`
}

// MapTransactionStatusToApp collapses the acquisition transaction state
// machine's finer-grained statuses to the app contract's status values
// (ACTIVE|PENDING|CANCELLED|FAILED); the app doesn't distinguish
// ACTION_REQUIRED from CONFIRM_REQUIRED.
func MapTransactionStatusToApp(status TransactionStatus) string {
	switch status {
	case StatusSubscribed, StatusCharged:
		return "ACTIVE"
	case StatusPending, StatusActionRequired, StatusConfirmRequired:
		return "PENDING"
	case StatusCancelled:
		return "CANCELLED"
	default:
		return "FAILED"
	}
}

// AppMarketplaceTenant is one tenant's storefront section in the marketplace
// response of GET /v1/app/catalog (no tenant filter).
type AppMarketplaceTenant struct {
	TenantKey  string               `json:"tenant_key"`
	TenantName string               `json:"tenant_name"`
	Products   []*AppCatalogProduct `json:"products"`
}

// AppSubscription is a single entry for GET /v1/app/subscriptions.
type AppSubscription struct {
	Ref            string    `json:"ref"`
	Tenant         string    `json:"tenant"`
	TenantName     string    `json:"tenant_name"`
	ProductSlug    string    `json:"product_slug"`
	ProductName    string    `json:"product_name"`
	Status         string    `json:"status"`
	Price          *float64  `json:"price,omitempty"`
	Currency       string    `json:"currency,omitempty"`
	BillingCycle   string    `json:"billing_cycle,omitempty"`
	NextChargeHint string    `json:"next_charge_hint,omitempty"`
	StartedAt      time.Time `json:"started_at"`
}
