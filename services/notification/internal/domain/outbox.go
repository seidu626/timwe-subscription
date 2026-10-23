package domain

import "time"

type OutboxJob struct {
	JobID           string
	TenantID        *string
	ChannelID       *string
	SubscriptionID  int64
	ContentItemID   int64
	Attempt         int
	PlannedSendAt   time.Time
	MessageText     string
	MSISDN          string
	EntryChannel    string
	ProductID       int
	PartnerRoleID   int
	DeliveryChannel string
	// SkipReason is set by the claim query when the job must not be sent
	// (subscription inactive, or planned too long ago); empty means send.
	SkipReason string
}

// DeliveryCheck is a SENT gateway job awaiting a handset delivery verdict,
// claimed by the delivery poller.
type DeliveryCheck struct {
	JobID             string
	TenantID          string
	MSISDN            string
	ProviderMessageID string
	SentAt            time.Time
}
