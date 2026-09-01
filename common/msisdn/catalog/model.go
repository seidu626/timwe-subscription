package catalog

import "time"

const (
	RegionGhana   = "GH"
	RegionNigeria = "NG"

	StatusPending  = "PENDING"
	StatusVerified = "VERIFIED"
	StatusInvalid  = "INVALID"
	StatusError    = "ERROR"

	SourceBatchGenerated = "BATCH_GENERATED"
	SourceDNDImport      = "DND_IMPORT"
	SourceManual         = "MANUAL"
)

// Record is the privacy-minimal MSISDN catalog entry. MADAPI KYC payloads are
// deliberately excluded; only the verification verdict and provider reference
// are retained.
type Record struct {
	ID                    int64      `json:"id"`
	TenantID              string     `json:"tenant_id"`
	MSISDN                string     `json:"msisdn"`
	Region                string     `json:"region"`
	Telco                 string     `json:"telco"`
	VerificationStatus    string     `json:"verification_status"`
	DND                   bool       `json:"dnd"`
	Invalid               bool       `json:"invalid"`
	InvalidReason         *string    `json:"invalid_reason,omitempty"`
	Source                string     `json:"source"`
	GenerationBatchID     *string    `json:"generation_batch_id,omitempty"`
	VerificationProvider  *string    `json:"verification_provider,omitempty"`
	VerificationReference *string    `json:"verification_reference,omitempty"`
	VerificationAttempts  int        `json:"verification_attempts"`
	VerifiedAt            *time.Time `json:"verified_at,omitempty"`
	LastVerificationAt    *time.Time `json:"last_verification_at,omitempty"`
	LastError             *string    `json:"last_error,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ListFilter struct {
	TenantID           string
	Query              string
	Region             string
	Telco              string
	VerificationStatus string
	DND                *bool
	Invalid            *bool
	Limit              int
	Offset             int
}

type Stats struct {
	Total    int64 `json:"total"`
	Ready    int64 `json:"ready"`
	Verified int64 `json:"verified"`
	Pending  int64 `json:"pending"`
	DND      int64 `json:"dnd"`
	Invalid  int64 `json:"invalid"`
	Errors   int64 `json:"errors"`
}

type VerificationResult struct {
	Status    string
	Reference string
	Reason    string
	Provider  string
	Retryable bool
}
