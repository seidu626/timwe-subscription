package domain

import "time"

type ListResponse struct {
	Data        []*Subscription `json:"data"`
	TotalCount  int             `json:"totalCount"`
	PageSize    int             `json:"pageSize"`
	Page        int             `json:"page"`
	TotalPages  int             `json:"totalPages"`
	HasPrevPage bool            `json:"hasPrevPage"`
	HasNextPage bool            `json:"hasNextPage"`
}

type Subscription struct {
	Id                  int        `json:"id"`
	PartnerRoleId       string     `json:"partnerRoleId"`
	UserIdentifier      string     `json:"userIdentifier"`
	UserIdentifierType  string     `json:"userIdentifierType"`
	ProductId           string     `json:"productId"`
	Mcc                 string     `json:"mcc"`
	Mnc                 string     `json:"mnc"`
	EntryChannel        string     `json:"entryChannel"`
	LargeAccount        string     `json:"largeAccount"`
	SubKeyword          string     `json:"subKeyword"`
	TrackingId          string     `json:"trackingId"`
	ClientIp            string     `json:"clientIp"`
	CampaignUrl         string     `json:"campaignUrl"`
	Status              string     `json:"status"`
	CancelReason        *string    `json:"cancelReason"`
	CancelSource        *string    `json:"cancelSource"`
	CreatedAt           string     `json:"createdAt"`
	StartDate           time.Time  `json:"startDate"`
	EndDate             *time.Time `json:"endDate"`
	TransactionAuthCode *string    `json:"transactionAuthCode"`
}

type SubscriptionRequest struct {
	PartnerRoleId      int    `json:"-"`
	UserIdentifier     string `json:"userIdentifier"`
	UserIdentifierType string `json:"userIdentifierType"`
	ProductId          int    `json:"productId"`
	Mcc                string `json:"mcc"`
	Mnc                string `json:"mnc"`
	EntryChannel       string `json:"entryChannel"`
	LargeAccount       string `json:"largeAccount"`
	SubKeyword         string `json:"subKeyword"`
	TrackingId         string `json:"trackingId"`
	ClientIp           string `json:"clientIp"`
	CampaignUrl        string `json:"campaignUrl"`
}

// SubscribeResponse represents the expected response from the external service.
type SubscribeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SubscriptionConfirmationRequest struct {
	PartnerRoleId       int    `json:"-"`
	UserIdentifier      string `json:"userIdentifier"`
	UserIdentifierType  string `json:"userIdentifierType"`
	ProductId           int    `json:"productId"`
	Mcc                 string `json:"mcc"`
	Mnc                 string `json:"mnc"`
	EntryChannel        string `json:"entryChannel"`
	ClientIp            string `json:"clientIp"`
	TransactionAuthCode string `json:"transactionAuthCode"`
}

type UnsubscriptionRequest struct {
	PartnerRoleId         int    `json:"-"`
	UserIdentifier        string `json:"userIdentifier"`
	UserIdentifierType    string `json:"userIdentifierType"`
	ProductId             int    `json:"productId"`
	Mcc                   string `json:"mcc"`
	Mnc                   string `json:"mnc"`
	EntryChannel          string `json:"entryChannel"`
	LargeAccount          string `json:"largeAccount"`
	SubKeyword            string `json:"subKeyword"`
	TrackingId            string `json:"trackingId"`
	ClientIp              string `json:"clientIp"`
	ControlKeyword        string `json:"controlKeyword"`
	ControlServiceKeyword string `json:"controlServiceKeyword"`
	SubId                 int    `json:"subId"`
	CancelReason          int    `json:"cancelReason"`
	CancelSource          int    `json:"cancelSource"`
}

type GetStatusRequest struct {
	PartnerRoleId         int    `json:"-"`
	UserIdentifier        string `json:"userIdentifier"`
	UserIdentifierType    string `json:"userIdentifierType"`
	ProductId             int    `json:"productId"`
	Mcc                   string `json:"mcc"`
	Mnc                   string `json:"mnc"`
	EntryChannel          string `json:"entryChannel"`
	ClientIp              string `json:"clientIp"`
	ControlKeyword        string `json:"controlKeyword"`
	ControlServiceKeyword string `json:"controlServiceKeyword"`
	SubId                 int    `json:"subId"`
}

type SubscriptionStatus struct {
	ProductId      int    `json:"productId"`
	UserIdentifier string `json:"userIdentifier"`
	Status         string `json:"status"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
}

// NotificationRequest represents inbound TIMWE webhook payload persisted to notifications.
type NotificationRequest struct {
	PartnerRole     int      `json:"partnerRole"`
	ExternalTxID    string   `json:"externalTxId"`
	ProductID       int      `json:"productId"`
	PricepointID    int      `json:"pricepointId"`
	MCC             string   `json:"mcc"`
	MNC             string   `json:"mnc"`
	MSISDN          string   `json:"msisdn"`
	LargeAccount    string   `json:"largeAccount"`
	TransactionUUID string   `json:"transactionUuid"`
	EntryChannel    string   `json:"entryChannel"`
	MessageType     string   `json:"messageType"`
	Message         string   `json:"message"`
	MnoDeliveryCode string   `json:"mnoDeliveryCode"`
	Tags            []string `json:"tags"`
	Type            string   `json:"type"`
}
