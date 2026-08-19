package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

const defaultNotificationAPIURL = "http://notification:8082"

// NotificationClient forwards locally observed subscription lifecycle events
// to notification-service, which owns template lookup and idempotent outbox
// creation.
type NotificationClient struct {
	baseURL    string
	trustToken string
	httpClient *http.Client
}

// NewNotificationClient creates the internal notification-service client.
// notification-service enforces the KrakenD gateway-trust marker on its
// partner-callback endpoints when GATEWAY_TRUST_REQUIRED=true, so this
// service-to-service client derives the same static token from the shared
// GATEWAY_TRUST_SECRET (see tenantctx.GatewayTrustToken).
func NewNotificationClient() *NotificationClient {
	baseURL := strings.TrimSpace(os.Getenv("NOTIFICATION_API_URL"))
	if baseURL == "" {
		baseURL = defaultNotificationAPIURL
	}
	trustToken := ""
	if secret := strings.TrimSpace(os.Getenv("GATEWAY_TRUST_SECRET")); secret != "" {
		trustToken = tenantctx.GatewayTrustToken(secret)
	}
	return &NotificationClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		trustToken: trustToken,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// NotifyUserOptin records an opt-in through notification-service. Reusing the
// provider external transaction ID lets notification-service deduplicate this
// handoff against a delayed carrier callback for the same provider request.
func (c *NotificationClient) NotifyUserOptin(ctx context.Context, route domain.TenantRouteContext, partnerRole int, notification *domain.NotificationRequest) error {
	if c == nil || c.httpClient == nil {
		return fmt.Errorf("notification client is not configured")
	}
	if strings.TrimSpace(route.TenantKey) == "" || strings.TrimSpace(route.ChannelKey) == "" {
		return fmt.Errorf("tenant_key and channel_key are required")
	}
	if partnerRole <= 0 {
		return fmt.Errorf("partner role is required")
	}
	if notification == nil {
		return fmt.Errorf("notification is required")
	}
	if strings.TrimSpace(notification.ExternalTxID) == "" {
		return fmt.Errorf("externalTxId is required")
	}

	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal user opt-in notification: %w", err)
	}

	endpoint, err := url.Parse(c.baseURL + "/api/v1/notification/user-optin/" + strconv.Itoa(partnerRole))
	if err != nil {
		return fmt.Errorf("build notification URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("tenant_key", route.TenantKey)
	query.Set("channel_key", route.ChannelKey)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create user opt-in notification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.trustToken != "" {
		req.Header.Set(tenantctx.HeaderGatewayTrust, c.trustToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call notification service: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification service returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
