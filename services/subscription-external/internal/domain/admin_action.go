package domain

import (
	"bytes"
	"encoding/json"
	"time"
)

type AdminActionOperation string

const (
	AdminActionOptin   AdminActionOperation = "optin"
	AdminActionOptout  AdminActionOperation = "optout"
	AdminActionConfirm AdminActionOperation = "confirm"
	AdminActionStatus  AdminActionOperation = "status"
)

type AdminSubscriptionActionRequest struct {
	TenantID              string            `json:"tenantId,omitempty"`
	TenantKey             string            `json:"tenantKey,omitempty"`
	ChannelID             string            `json:"channelId,omitempty"`
	ChannelKey            string            `json:"channelKey,omitempty"`
	MSISDN                string            `json:"msisdn"`
	ProductID             int               `json:"productId"`
	PartnerRoleID         int               `json:"partnerRoleId,omitempty"`
	UserIdentifierType    string            `json:"userIdentifierType,omitempty"`
	MCC                   string            `json:"mcc,omitempty"`
	MNC                   string            `json:"mnc,omitempty"`
	EntryChannel          string            `json:"entryChannel,omitempty"`
	LargeAccount          string            `json:"largeAccount,omitempty"`
	SubKeyword            string            `json:"subKeyword,omitempty"`
	TrackingID            string            `json:"trackingId,omitempty"`
	ClientIP              string            `json:"clientIp,omitempty"`
	CampaignURL           string            `json:"campaignUrl,omitempty"`
	ControlKeyword        string            `json:"controlKeyword,omitempty"`
	ControlServiceKeyword string            `json:"controlServiceKeyword,omitempty"`
	SubID                 int               `json:"subId,omitempty"`
	CancelReason          int               `json:"cancelReason,omitempty"`
	CancelSource          int               `json:"cancelSource,omitempty"`
	TransactionAuthCode   string            `json:"transactionAuthCode,omitempty"`
	ExternalTxID          string            `json:"externalTxId,omitempty"`
	AdminRequestID        string            `json:"adminRequestId,omitempty"`
	Headers               map[string]string `json:"headers,omitempty"`
}

type AdminActionCapturedRequest struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Body      json.RawMessage   `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
}

type AdminActionCapturedResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body"`
	Timestamp  *time.Time        `json:"timestamp,omitempty"`
	DurationMs int64             `json:"durationMs"`
}

type AdminSubscriptionActionLog struct {
	ID                 string               `json:"id"`
	TenantID           *string              `json:"tenantId,omitempty"`
	ChannelID          *string              `json:"channelId,omitempty"`
	Operation          AdminActionOperation `json:"operation"`
	MSISDN             string               `json:"msisdn"`
	ProductID          int                  `json:"productId"`
	PartnerRoleID      int                  `json:"partnerRoleId"`
	ExternalTxID       string               `json:"externalTxId,omitempty"`
	AdminRequestID     string               `json:"adminRequestId,omitempty"`
	RequestMethod      string               `json:"-"`
	RequestURL         string               `json:"-"`
	RequestHeaders     map[string]string    `json:"-"`
	RequestBody        json.RawMessage      `json:"-"`
	RequestTimestamp   time.Time            `json:"-"`
	ResponseStatusCode int                  `json:"-"`
	ResponseHeaders    map[string]string    `json:"-"`
	ResponseBody       json.RawMessage      `json:"-"`
	ResponseTimestamp  *time.Time           `json:"-"`
	ServiceResult      json.RawMessage      `json:"-"`
	ErrorPayload       json.RawMessage      `json:"-"`
	DurationMs         int64                `json:"-"`
	CreatedAt          time.Time            `json:"createdAt"`
}

type AdminSubscriptionActionDetail struct {
	ID             string                      `json:"id"`
	TenantID       *string                     `json:"tenantId,omitempty"`
	ChannelID      *string                     `json:"channelId,omitempty"`
	Operation      AdminActionOperation        `json:"operation"`
	MSISDN         string                      `json:"msisdn"`
	ProductID      int                         `json:"productId"`
	PartnerRoleID  int                         `json:"partnerRoleId"`
	ExternalTxID   string                      `json:"externalTxId,omitempty"`
	AdminRequestID string                      `json:"adminRequestId,omitempty"`
	Request        AdminActionCapturedRequest  `json:"request"`
	Response       AdminActionCapturedResponse `json:"response"`
	ServiceResult  json.RawMessage             `json:"serviceResult,omitempty"`
	Error          json.RawMessage             `json:"error,omitempty"`
	CreatedAt      time.Time                   `json:"createdAt"`
}

func (l *AdminSubscriptionActionLog) HasError() bool {
	trimmed := bytes.TrimSpace(l.ErrorPayload)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func (l *AdminSubscriptionActionLog) ErrorMessage() string {
	if !l.HasError() {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(l.ErrorPayload, &payload); err != nil {
		return ""
	}
	if msg, ok := payload["message"].(string); ok {
		return msg
	}
	return ""
}

func (l *AdminSubscriptionActionLog) ToDetail() AdminSubscriptionActionDetail {
	return AdminSubscriptionActionDetail{
		ID:             l.ID,
		TenantID:       l.TenantID,
		ChannelID:      l.ChannelID,
		Operation:      l.Operation,
		MSISDN:         l.MSISDN,
		ProductID:      l.ProductID,
		PartnerRoleID:  l.PartnerRoleID,
		ExternalTxID:   l.ExternalTxID,
		AdminRequestID: l.AdminRequestID,
		Request: AdminActionCapturedRequest{
			Method:    l.RequestMethod,
			URL:       l.RequestURL,
			Headers:   l.RequestHeaders,
			Body:      l.RequestBody,
			Timestamp: l.RequestTimestamp,
		},
		Response: AdminActionCapturedResponse{
			StatusCode: l.ResponseStatusCode,
			Headers:    l.ResponseHeaders,
			Body:       l.ResponseBody,
			Timestamp:  l.ResponseTimestamp,
			DurationMs: l.DurationMs,
		},
		ServiceResult: l.ServiceResult,
		Error:         l.ErrorPayload,
		CreatedAt:     l.CreatedAt,
	}
}

type AdminActionLogFilter struct {
	TenantID       string
	Operation      AdminActionOperation
	MSISDN         string
	ExternalTxID   string
	AdminRequestID string
	ProductID      int
	StartDate      time.Time
	EndDate        time.Time
	Result         string
	SortBy         string
	SortDir        string
	Page           int
	PageSize       int
}

type AdminActionLogSummary struct {
	ID                 string               `json:"id"`
	TenantID           *string              `json:"tenantId,omitempty"`
	ChannelID          *string              `json:"channelId,omitempty"`
	Operation          AdminActionOperation `json:"operation"`
	MSISDN             string               `json:"msisdn"`
	ProductID          int                  `json:"productId"`
	PartnerRoleID      int                  `json:"partnerRoleId"`
	ExternalTxID       string               `json:"externalTxId,omitempty"`
	AdminRequestID     string               `json:"adminRequestId,omitempty"`
	ResponseStatusCode int                  `json:"responseStatusCode"`
	DurationMs         int64                `json:"durationMs"`
	CreatedAt          time.Time            `json:"createdAt"`
	HasError           bool                 `json:"hasError"`
	ErrorMessage       string               `json:"errorMessage,omitempty"`
}

type AdminActionLogListResponse struct {
	Data       []AdminActionLogSummary `json:"data"`
	TotalCount int64                   `json:"totalCount"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"pageSize"`
	TotalPages int                     `json:"totalPages"`
}
