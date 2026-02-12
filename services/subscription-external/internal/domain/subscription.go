package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BatchOptinRequest struct {
	Telco        string   `json:"telco"`
	Count        int      `json:"count"`
	EntryChannel string   `json:"entry_channel"`
	MSISDNS      []string `json:"msisdns,omitempty"` // if provided, skip generation
	ProductIds   []string `json:"product_ids,omitempty"`
}

type BackfillRequest struct {
	EntryChannel  string   `json:"entry_channel"`            // Single channel (legacy support)
	EntryChannels []string `json:"entry_channels,omitempty"` // Multiple channels for rotation
	Telco         string   `json:"telco"`
	ProductIds    []string `json:"product_ids"`
	MSISDNS       []string `json:"msisdns,omitempty"`     // optional explicit list to include
	StartIndex    int      `json:"start_index,omitempty"` // 0 or -1 means use full list
	EndIndex      int      `json:"end_index,omitempty"`
}

// GetNextEntryChannel returns the next entry channel in rotation
func (r *BackfillRequest) GetNextEntryChannel() string {
	// If no channels configured, fall back to single channel
	if len(r.EntryChannels) == 0 {
		return r.EntryChannel
	}

	// Use a simple rotation based on the current time
	// This provides a basic distribution without requiring state management
	index := time.Now().UnixNano() % int64(len(r.EntryChannels))
	return r.EntryChannels[index]
}

// BatchOptinResponse represents the response for batch opt-in requests
type BatchOptinResponse struct {
	Total        int                     `json:"total"`
	Successful   int                     `json:"successful"`
	Failed       int                     `json:"failed"`
	ErrorDetails *map[string]interface{} `json:"errorDetails,omitempty"` // Details of the first error encountered
}

type OptinRequest struct {
	Telco        string   `json:"telco"`
	EntryChannel string   `json:"entry_channel"`
	Msisdn       string   `json:"msisdn"`
	ProductIds   []string `json:"product_ids"`
}

type MTRequest struct {
	ProductID          int    `json:"productId"`
	PricepointID       int    `json:"pricepointId"`
	MCC                string `json:"mcc"`
	MNC                string `json:"mnc"`
	UserIdentifier     string `json:"userIdentifier"`
	UserIdentifierType string `json:"userIdentifierType"`
	EntryChannel       string `json:"entryChannel"`
	SubKeyword         string `json:"subKeyword"`
	LargeAccount       string `json:"largeAccount"`
	CampaignUrl        string `json:"campaignUrl"`
	SendDate           string `json:"sendDate"`
	Priority           string `json:"priority"`
	Timezone           string `json:"timezone"`
	Context            string `json:"context"`
	MoTransactionUUID  string `json:"moTransactionUUID"`
}

type MTResponse struct {
	ResponseData map[string]interface{} `json:"responseData"`
	Message      string                 `json:"message"`
	InError      bool                   `json:"inError"`
	RequestID    string                 `json:"requestId"`
	Code         string                 `json:"code"`
}

// Custom error types for better error handling
type MTResponseError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

func (e *MTResponseError) Error() string {
	return fmt.Sprintf("MT response error [%s]: %s", e.Code, e.Message)
}

type ChargeRequest struct {
	ProductID    int    `json:"productId"`
	PricepointID int    `json:"pricepointId"`
	MCC          string `json:"mcc"`
	MNC          string `json:"mnc"`
	MSISDN       string `json:"msisdn"`
	ShortCode    string `json:"shortCode"`
	Context      string `json:"context"`
	Channel      string `json:"channel"`
}

type ChargeResponse struct {
	ResponseData map[string]interface{} `json:"responseData"`
	Message      string                 `json:"message"`
	InError      bool                   `json:"inError"`
	RequestID    string                 `json:"requestId"`
	Code         string                 `json:"code"`
}

func MapMTRequestToSubscriptionRequest(mtReq MTRequest, transactionId string, partnerRoleId int, clientIp, campaignUrl string) SubscriptionRequest {
	return SubscriptionRequest{
		TransactionId:      transactionId,
		PartnerRoleId:      partnerRoleId,        // Set partner role explicitly
		UserIdentifier:     mtReq.UserIdentifier, // Map UserIdentifier to UserIdentifier
		UserIdentifierType: mtReq.UserIdentifierType,
		ProductId:          mtReq.ProductID,
		Mcc:                &mtReq.MCC,               // Map MCC as pointer
		Mnc:                &mtReq.MNC,               // Map MNC as pointer
		EntryChannel:       &mtReq.EntryChannel,      // Map EntryChannel as pointer
		LargeAccount:       &mtReq.LargeAccount,      // Map directly as pointer
		SubKeyword:         &mtReq.SubKeyword,        // Map SubKeyword as pointer
		TrackingId:         &mtReq.MoTransactionUUID, // Map MoTransactionUUID as pointer
		ClientIp:           &clientIp,                // Set Client IP as pointer
		CampaignUrl:        &campaignUrl,             // Set Campaign URL as pointer
	}
}

// MapChargeToNotification maps a ChargeRequest to a NotificationRequest.
func MapChargeToNotification(chargeReq ChargeRequest, partnerRole int) NotificationRequest {
	txId := uuid.New().String()
	return NotificationRequest{
		PartnerRole:     partnerRole,
		ExternalTxID:    txId,
		ProductID:       chargeReq.ProductID,
		PricepointID:    chargeReq.PricepointID,
		MCC:             chargeReq.MCC,
		MNC:             chargeReq.MNC,
		MSISDN:          chargeReq.MSISDN,
		LargeAccount:    chargeReq.ShortCode,
		TransactionUUID: txId,
		EntryChannel:    chargeReq.Channel,
		MessageType:     "Charge",
		Message:         chargeReq.Context,
		Tags:            []string{"billing", "charge"},
		Type:            "CHARGE",
	}
}

type NotificationRequest struct {
	PartnerRole     int      `json:"partnerRole"`
	ExternalTxID    string   `json:"externalTxId"`
	ProductID       int      `json:"productId"`
	PricepointID    int      `json:"pricepointId"`
	MCC             string   `json:"mcc"`
	MNC             string   `json:"mnc"`
	MSISDN          string   `json:"msisdn"`
	LargeAccount    string   `json:"largeAccount"`
	TransactionUUID string   `json:"transactionUUID"`
	EntryChannel    string   `json:"entryChannel,omitempty"`
	MessageType     string   `json:"messageType"`
	Message         string   `json:"message"`
	MnoDeliveryCode string   `json:"mnoDeliveryCode,omitempty"`
	Tags            []string `json:"tags"`
	Type            string   `json:"type"`
}

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
	Mcc                 *string    `json:"mcc"`          // Changed to pointer to handle NULL values
	Mnc                 *string    `json:"mnc"`          // Changed to pointer to handle NULL values
	EntryChannel        *string    `json:"entryChannel"` // Changed to pointer to handle NULL values
	LargeAccount        *string    `json:"largeAccount"` // Changed to pointer to handle NULL values
	SubKeyword          *string    `json:"subKeyword"`   // Changed to pointer to handle NULL values
	TrackingId          *string    `json:"trackingId"`   // Changed to pointer to handle NULL values
	ClientIp            *string    `json:"clientIp"`     // Changed to pointer to handle NULL values
	CampaignUrl         *string    `json:"campaignUrl"`  // Changed to pointer to handle NULL values
	Status              string     `json:"status"`
	CancelReason        *string    `json:"cancelReason"`
	CancelSource        *string    `json:"cancelSource"`
	CreatedAt           string     `json:"createdAt"`
	StartDate           time.Time  `json:"startDate"`
	EndDate             *time.Time `json:"endDate"`
	TransactionAuthCode *string    `json:"transactionAuthCode"`
}

type SubscriptionRequest struct {
	TransactionId      string  `json:"transactionId"`
	PartnerRoleId      int     `json:"-"`
	UserIdentifier     string  `json:"userIdentifier"`
	UserIdentifierType string  `json:"userIdentifierType"`
	ProductId          int     `json:"productId"`
	Mcc                *string `json:"mcc"` // Changed to pointer to handle NULL values
	Mnc                *string `json:"mnc"` // Changed to pointer to handle NULL values
	Status             string  `json:"status"`
	EntryChannel       *string `json:"entryChannel"` // Changed to pointer to handle NULL values
	LargeAccount       *string `json:"largeAccount"` // Changed to pointer to handle NULL values
	SubKeyword         *string `json:"subKeyword"`   // Changed to pointer to handle NULL values
	TrackingId         *string `json:"trackingId"`   // Changed to pointer to handle NULL values
	ClientIp           *string `json:"clientIp"`     // Changed to pointer to handle NULL values
	CampaignUrl        *string `json:"campaignUrl"`  // Changed to pointer to handle NULL values
}

// SubscribeResponse represents the expected response from the external service.
type SubscribeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SubscriptionConfirmationRequest struct {
	PartnerRoleId       int     `json:"-"`
	UserIdentifier      string  `json:"userIdentifier"`
	UserIdentifierType  string  `json:"userIdentifierType"`
	ProductId           int     `json:"productId"`
	Mcc                 *string `json:"mcc"`          // Changed to pointer to handle NULL values
	Mnc                 *string `json:"mnc"`          // Changed to pointer to handle NULL values
	EntryChannel        *string `json:"entryChannel"` // Changed to pointer to handle NULL values
	ClientIp            *string `json:"clientIp"`     // Changed to pointer to handle NULL values
	TransactionAuthCode string  `json:"transactionAuthCode"`
}

type UnsubscriptionRequest struct {
	PartnerRoleId         int     `json:"-"`
	UserIdentifier        string  `json:"userIdentifier"`
	UserIdentifierType    string  `json:"userIdentifierType"`
	ProductId             int     `json:"productId"`
	Mcc                   *string `json:"mcc"`          // Changed to pointer to handle NULL values
	Mnc                   *string `json:"mnc"`          // Changed to pointer to handle NULL values
	EntryChannel          *string `json:"entryChannel"` // Changed to pointer to handle NULL values
	LargeAccount          *string `json:"largeAccount"` // Changed to pointer to handle NULL values
	SubKeyword            *string `json:"subKeyword"`   // Changed to pointer to handle NULL values
	TrackingId            *string `json:"trackingId"`   // Changed to pointer to handle NULL values
	ClientIp              *string `json:"clientIp"`     // Changed to pointer to handle NULL values
	ControlKeyword        string  `json:"controlKeyword"`
	ControlServiceKeyword string  `json:"controlServiceKeyword"`
	SubId                 int     `json:"subId"`
	CancelReason          int     `json:"cancelReason"`
	CancelSource          int     `json:"cancelSource"`
}

type GetStatusRequest struct {
	PartnerRoleId         int     `json:"-"`
	UserIdentifier        string  `json:"userIdentifier"`
	UserIdentifierType    string  `json:"userIdentifierType"`
	ProductId             int     `json:"productId"`
	Mcc                   *string `json:"mcc"`          // Changed to pointer to handle NULL values
	Mnc                   *string `json:"mnc"`          // Changed to pointer to handle NULL values
	EntryChannel          *string `json:"entryChannel"` // Changed to pointer to handle NULL values
	ClientIp              *string `json:"clientIp"`     // Changed to pointer to handle NULL values
	ControlKeyword        string  `json:"controlKeyword"`
	ControlServiceKeyword string  `json:"controlServiceKeyword"`
	SubId                 int     `json:"subId"`
}

type SubscriptionStatus struct {
	ProductId      int    `json:"productId"`
	UserIdentifier string `json:"userIdentifier"`
	Status         string `json:"status"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
}

// InvalidMSISDNLog represents a log entry for invalid MSISDN responses
type InvalidMSISDNLog struct {
	ID                 int       `json:"id"`
	MSISDN             string    `json:"msisdn"`
	ProductID          *int      `json:"productId,omitempty"`
	PricepointID       *int      `json:"pricepointId,omitempty"`
	PartnerRoleID      *int      `json:"partnerRoleId,omitempty"`
	EntryChannel       string    `json:"entryChannel,omitempty"`
	RequestID          string    `json:"requestId,omitempty"`
	ResponseCode       string    `json:"responseCode,omitempty"`
	ResponseMessage    string    `json:"responseMessage,omitempty"`
	SubscriptionResult string    `json:"subscriptionResult,omitempty"`
	SubscriptionError  string    `json:"subscriptionError,omitempty"`
	ExternalTxID       string    `json:"externalTxId,omitempty"`
	TransactionID      string    `json:"transactionId,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}
