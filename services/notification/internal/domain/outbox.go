package domain

import "time"

type OutboxJob struct {
	JobID          string
	TenantID       *string
	ChannelID      *string
	SubscriptionID int64
	ContentItemID  int64
	Attempt        int
	PlannedSendAt  time.Time
	MessageText    string
	MSISDN         string
	EntryChannel   string
	ProductID      int
	PartnerRoleID  int
}
