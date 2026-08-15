package domain

import "time"

// AppFeedItem is a single Dayline app feed entry: content already delivered
// to the subscriber's msisdn via message_outbox, enriched with product
// context and read state. Field names are snake_case per
// docs/dayline-app-api-contract.md.
type AppFeedItem struct {
	ID          int64     `json:"id"`
	ProductSlug string    `json:"product_slug"`
	ProductName string    `json:"product_name"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	ContentKind string    `json:"content_kind"`
	LinkURL     *string   `json:"link_url"`
	CTALabel    *string   `json:"cta_label"`
	PublishedAt time.Time `json:"published_at"`
	Read        bool      `json:"read"`
}

// AppFeedListResponse wraps GET /v1/app/feed.
type AppFeedListResponse struct {
	Items []AppFeedItem `json:"items"`
}

// AppDeviceRequest is the body of POST /v1/app/devices.
type AppDeviceRequest struct {
	FCMToken string `json:"fcm_token"`
	Platform string `json:"platform"`
}

// AppNotificationPrefRequest is the body of PUT /v1/app/notification-prefs.
type AppNotificationPrefRequest struct {
	ProductSlug string `json:"product_slug"`
	Channel     string `json:"channel"`
}
