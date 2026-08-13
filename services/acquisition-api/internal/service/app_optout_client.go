package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// AppOptoutClient calls subscription-external's partner opt-out endpoint
// directly in-cluster (bypassing KrakenD), the same gateway-trust pattern
// acquisition-api already uses for opt-in via TIMWEClientImpl. It backs
// DELETE /v1/app/subscriptions/{ref}.
type AppOptoutClient struct {
	baseURL            string
	gatewayTrustSecret string
	client             *fasthttp.Client
	logger             *zap.Logger
}

// NewAppOptoutClient creates a new AppOptoutClient. Returns nil if baseURL
// or gatewayTrustSecret is empty, so callers can treat a nil client as "opt-out
// is not configured" rather than issuing a call that will always fail.
func NewAppOptoutClient(baseURL, gatewayTrustSecret string, logger *zap.Logger) *AppOptoutClient {
	baseURL = strings.TrimSpace(baseURL)
	gatewayTrustSecret = strings.TrimSpace(gatewayTrustSecret)
	if baseURL == "" || gatewayTrustSecret == "" {
		return nil
	}
	return &AppOptoutClient{
		baseURL:            baseURL,
		gatewayTrustSecret: gatewayTrustSecret,
		client:             &fasthttp.Client{},
		logger:             logger,
	}
}

type appUnsubscriptionRequest struct {
	UserIdentifier     string `json:"userIdentifier"`
	UserIdentifierType string `json:"userIdentifierType"`
	ProductId          int    `json:"productId"`
}

// Optout triggers subscription-external's existing opt-out path for
// msisdn+productID under the given tenant/channel.
func (c *AppOptoutClient) Optout(tenantKey, channelKey, msisdn string, productID int) error {
	body, err := json.Marshal(appUnsubscriptionRequest{
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		ProductId:          productID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal optout request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/v1/subscription-external/partners/optout?tenant_key=%s&channel_key=%s",
		strings.TrimRight(c.baseURL, "/"), url.QueryEscape(tenantKey), url.QueryEscape(channelKey))

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(reqURL)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.Header.Set(tenantctx.HeaderGatewayTrust, tenantctx.GatewayTrustToken(c.gatewayTrustSecret))
	req.SetBody(body)

	if err := c.client.DoTimeout(req, resp, 15*time.Second); err != nil {
		return fmt.Errorf("optout request failed: %w", err)
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return fmt.Errorf("optout request returned status %d", resp.StatusCode())
	}
	return nil
}
