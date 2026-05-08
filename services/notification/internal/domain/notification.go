package domain

import "time"

type ListResponse struct {
	Data        []*Notification `json:"data"`
	TotalCount  int             `json:"totalCount"`
	PageSize    int             `json:"pageSize"`
	Page        int             `json:"page"`
	TotalPages  int             `json:"totalPages"`
	HasPrevPage bool            `json:"hasPrevPage"`
	HasNextPage bool            `json:"hasNextPage"`
}

type Notification struct {
	ID              int       `gorm:"primary_key" json:"id"`
	TenantID        *string   `json:"tenantId,omitempty"`
	ChannelID       *string   `json:"channelId,omitempty"`
	PartnerRole     int       `json:"partnerRole"`
	ExternalTxID    string    `json:"externalTxId"`
	ProductID       int       `json:"productId"`
	PricepointID    int       `json:"pricepointId"`
	MCC             string    `json:"mcc"`
	MNC             string    `json:"mnc"`
	MSISDN          string    `json:"msisdn"`
	LargeAccount    string    `json:"largeAccount"`
	TransactionUUID string    `json:"transactionUUID"`
	EntryChannel    string    `json:"entryChannel,omitempty"`
	MessageType     string    `json:"messageType"`
	Message         string    `json:"message"`
	MnoDeliveryCode string    `json:"mnoDeliveryCode,omitempty"`
	Tags            []string  `json:"tags"`
	CreatedAt       time.Time `json:"createdAt"`
	Type            *string   `json:"type"`
}

type NotificationRequest struct {
	TenantID        *string  `json:"tenantId,omitempty"`
	ChannelID       *string  `json:"channelId,omitempty"`
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
