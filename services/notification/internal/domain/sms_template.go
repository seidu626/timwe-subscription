package domain

import (
	"strconv"
	"strings"
	"time"
)

const UserOptinEvent = "USER_OPTIN"
const maxSMSMessageRunes = 480

// SMSTemplate configures an event message for one tenant and product.
type SMSTemplate struct {
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenantId"`
	ProductID int       `json:"productId"`
	EventType string    `json:"eventType"`
	Enabled   bool      `json:"enabled"`
	Template  string    `json:"template"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SMSTemplateUpsert contains the mutable template fields accepted by the API.
type SMSTemplateUpsert struct {
	EventType string `json:"eventType"`
	Enabled   bool   `json:"enabled"`
	Template  string `json:"template"`
}

// RenderSMSTemplate substitutes supported placeholders without exposing more
// than the subscriber's final four digits and caps the rendered message.
func RenderSMSTemplate(template string, notification *NotificationRequest) string {
	msisdnRunes := []rune(notification.MSISDN)
	lastFour := notification.MSISDN
	if len(msisdnRunes) > 4 {
		lastFour = string(msisdnRunes[len(msisdnRunes)-4:])
	}
	rendered := strings.NewReplacer(
		"{{product_id}}", strconv.Itoa(notification.ProductID),
		"{{large_account}}", notification.LargeAccount,
		"{{msisdn}}", lastFour,
	).Replace(template)
	runes := []rune(rendered)
	if len(runes) > maxSMSMessageRunes {
		return string(runes[:maxSMSMessageRunes])
	}
	return rendered
}
