package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/sony/gobreaker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// TIMWEConfig holds configuration for subscription-external client.
type TIMWEConfig struct {
	BaseURL          string
	APIKey           string
	PSK              string
	PartnerRoleID    string
	PartnerServiceID string
	MCC              string
	MNC              string
	Timeout          time.Duration
	MaxConnections   int
	DialTimeout      time.Duration
	MaxConnDuration  time.Duration
	IdleConnTimeout  time.Duration
	// Retry settings
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	// Circuit breaker settings
	CBMaxRequests          uint32
	CBTimeout              time.Duration
	CBInterval             time.Duration
	CBMinRequests          uint32
	CBFailureRateThreshold float64
	CBConsecutiveFailures  uint32
	TrustedServiceSecret   string
	ServiceID              string
}

// DefaultTIMWEConfig returns default TIMWE configuration
func DefaultTIMWEConfig() *TIMWEConfig {
	return &TIMWEConfig{
		BaseURL:                "http://localhost:8080",
		Timeout:                30 * time.Second,
		MaxConnections:         500,
		DialTimeout:            10 * time.Second,
		MaxConnDuration:        60 * time.Second,
		IdleConnTimeout:        30 * time.Second,
		MaxRetries:             3,
		RetryBaseDelay:         200 * time.Millisecond,
		RetryMaxDelay:          5 * time.Second,
		CBMaxRequests:          100,
		CBTimeout:              15 * time.Second,
		CBInterval:             0,
		CBMinRequests:          50,
		CBFailureRateThreshold: 0.8,
		CBConsecutiveFailures:  10,
	}
}

// TIMWEClientImpl implements the TIMWEClient interface with real API calls
type TIMWEClientImpl struct {
	config         *TIMWEConfig
	client         *fasthttp.Client
	circuitBreaker *gobreaker.TwoStepCircuitBreaker
	logger         *zap.Logger
}

// MTRequest represents subscription-external MT request payload.
type MTRequest struct {
	ProductID         int    `json:"productId"`
	PricepointID      int    `json:"pricepointId,omitempty"`
	MCC               string `json:"mcc"`
	MNC               string `json:"mnc"`
	MSISDN            string `json:"msisdn"`
	SubKeyword        string `json:"subKeyword,omitempty"`
	LargeAccount      string `json:"largeAccount,omitempty"`
	CampaignUrl       string `json:"campaignUrl,omitempty"`
	SendDate          string `json:"sendDate,omitempty"`
	Priority          string `json:"priority,omitempty"`
	Timezone          string `json:"timezone,omitempty"`
	Context           string `json:"context,omitempty"`
	MoTransactionUUID string `json:"moTransactionUUID"`
	ChannelID         string `json:"channelId,omitempty"`
	ChannelKey        string `json:"channelKey,omitempty"`
}

// MTResponse represents the TIMWE MT response
type MTResponse struct {
	ResponseData map[string]interface{} `json:"responseData"`
	Message      string                 `json:"message"`
	InError      bool                   `json:"inError"`
	RequestID    string                 `json:"requestId"`
	Code         string                 `json:"code"`
}

type outboundRequestMeta struct {
	Operation string
	MSISDN    string
	ProductID int
	Headers   map[string]string
}

// ConfirmRequest represents the TIMWE subscription confirmation request
type ConfirmRequest struct {
	UserIdentifier      string `json:"userIdentifier"`
	UserIdentifierType  string `json:"userIdentifierType"`
	ProductID           int    `json:"productId"`
	MCC                 string `json:"mcc,omitempty"`
	MNC                 string `json:"mnc,omitempty"`
	EntryChannel        string `json:"entryChannel,omitempty"`
	ClientIP            string `json:"clientIp,omitempty"`
	TransactionAuthCode string `json:"transactionAuthCode"`
}

// NewTIMWEClient creates a new TIMWE client with actual API integration
func NewTIMWEClient(logger *zap.Logger) *TIMWEClientImpl {
	return NewTIMWEClientWithConfig(DefaultTIMWEConfig(), logger)
}

// NewTIMWEClientWithConfig creates a new TIMWE client with provided configuration
func NewTIMWEClientWithConfig(config *TIMWEConfig, logger *zap.Logger) *TIMWEClientImpl {
	// Create HTTP client with configured timeouts
	client := &fasthttp.Client{
		MaxConnsPerHost:               config.MaxConnections,
		MaxIdleConnDuration:           config.IdleConnTimeout,
		ReadTimeout:                   config.Timeout,
		WriteTimeout:                  config.Timeout,
		MaxConnWaitTimeout:            config.DialTimeout,
		MaxResponseBodySize:           10 * 1024 * 1024, // 10MB
		DisableHeaderNamesNormalizing: false,
	}

	// Create circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "timwe-api",
		MaxRequests: config.CBMaxRequests,
		Timeout:     config.CBTimeout,
		Interval:    config.CBInterval,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < config.CBMinRequests {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= config.CBFailureRateThreshold ||
				counts.ConsecutiveFailures >= config.CBConsecutiveFailures
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("TIMWE circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}
	cb := gobreaker.NewTwoStepCircuitBreaker(cbSettings)

	return &TIMWEClientImpl{
		config:         config,
		client:         client,
		circuitBreaker: cb,
		logger:         logger,
	}
}

// OptIn calls subscription-external opt-in endpoint.
func (c *TIMWEClientImpl) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return c.optIn(msisdn, productID, entryChannel, trackingFields, partnerRoleID, TenantSubscriptionContext{})
}

func (c *TIMWEClientImpl) OptInWithTenant(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string, tenant TenantSubscriptionContext) (*TIMWEResponse, error) {
	return c.optIn(msisdn, productID, entryChannel, trackingFields, partnerRoleID, tenant)
}

func (c *TIMWEClientImpl) optIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string, tenant TenantSubscriptionContext) (*TIMWEResponse, error) {
	c.logger.Info("Subscription opt-in called",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID),
		zap.String("entry_channel", entryChannel),
	)

	txUUID := uuid.New().String()

	if strings.TrimSpace(entryChannel) == "" {
		entryChannel = "WEB"
	}

	// Build request payload for subscription-external partner MT endpoint.
	reqData := MTRequest{
		ProductID:         productID,
		MCC:               c.config.MCC,
		MNC:               c.config.MNC,
		MSISDN:            msisdn,
		MoTransactionUUID: txUUID,
	}
	if tenant.TenantID != "" && tenant.ChannelID != "" {
		reqData.ChannelID = tenant.ChannelID
	}

	// Add tracking fields as context
	if len(trackingFields) > 0 {
		contextData, _ := json.Marshal(trackingFields)
		reqData.Context = string(contextData)
	}

	// partnerRoleID is intentionally ignored here - subscription-external owns TIMWE role selection.
	_ = partnerRoleID
	url := fmt.Sprintf("%s/api/external/v1/%s/mt", strings.TrimRight(c.config.BaseURL, "/"), strings.ToUpper(entryChannel))

	// Marshal request body
	requestBody, err := json.Marshal(reqData)
	if err != nil {
		c.logger.Error("Failed to marshal request data", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	// Log outbound request payload for debugging.
	var requestBodyMap map[string]interface{}
	_ = json.Unmarshal(requestBody, &requestBodyMap)
	c.logger.Info("Subscription opt-in outbound request",
		zap.String("url", url),
		zap.String("method", "POST"),
		zap.Any("body", requestBodyMap),
		zap.String("request_tracking_id", txUUID),
		zap.String("raw_body", string(requestBody)),
	)

	headers := map[string]string{}
	if tenant.TenantID != "" {
		signed, signErr := c.signedTenantHeaders("POST", fmt.Sprintf("/api/external/v1/%s/mt", strings.ToUpper(entryChannel)), requestBody, tenant)
		if signErr != nil {
			return nil, signErr
		}
		headers = signed
	}

	// Execute with circuit breaker
	done, cbErr := c.circuitBreaker.Allow()
	if cbErr != nil {
		c.logger.Error("Circuit breaker is open", zap.Error(cbErr))
		return nil, fmt.Errorf("circuit breaker open: %w", cbErr)
	}

	// Execute request with retries.
	mtResp, err := c.sendMTRequestWithRetry(url, requestBody, outboundRequestMeta{
		Operation: "optin",
		MSISDN:    msisdn,
		ProductID: productID,
		Headers:   headers,
	})
	if err != nil {
		done(false) // Mark as failure for circuit breaker
		return nil, err
	}
	done(true) // Mark as success

	// Convert MTResponse to TIMWEResponse
	response := &TIMWEResponse{
		Success:       !mtResp.InError,
		TransactionID: txUUID,
		Status:        mtResp.Code,
		Message:       sanitizeTIMWEMessage(mtResp.Message),
	}

	// Extract transaction auth code if present in response data
	if mtResp.ResponseData != nil {
		if authCode, ok := mtResp.ResponseData["transactionAuthCode"].(string); ok {
			response.TransactionAuthCode = authCode
		}
		if txID, ok := mtResp.ResponseData["transactionId"].(string); ok {
			response.TransactionID = txID
		}
	}

	// Determine if confirmation is required
	// Common TIMWE codes that require confirmation:
	// - OPTIN_PIN_WAITING: PIN required
	// - OPTIN_WAITING: Waiting for confirmation
	switch mtResp.Code {
	case "OPTIN_PIN_WAITING", "OPTIN_WAITING", "WAITING_FOR_CONFIRMATION":
		response.RequiresConfirm = true
	case "SUBSCRIBED", "SUCCESS", "OPTIN_SUCCESS":
		response.Success = true
		response.RequiresConfirm = false
	default:
		// Check for error codes
		if mtResp.InError {
			response.Success = false
		}
	}

	if optInResultRequiresConfirm(mtResp) {
		response.Success = true
		response.RequiresConfirm = true
	}

	c.logger.Info("Subscription opt-in response",
		zap.String("msisdn", msisdn),
		zap.String("code", mtResp.Code),
		zap.Bool("success", response.Success),
		zap.Bool("requires_confirm", response.RequiresConfirm),
	)

	return response, nil
}

func optInResultRequiresConfirm(mtResp *MTResponse) bool {
	if mtResp == nil || mtResp.ResponseData == nil {
		return false
	}

	for _, key := range []string{"subscriptionResult", "status", "subscriptionStatus"} {
		raw, ok := mtResp.ResponseData[key]
		if !ok {
			continue
		}

		value, ok := raw.(string)
		if !ok {
			continue
		}

		switch strings.ToUpper(value) {
		case "OPTIN_PREACTIVE_WAIT_CONF", "OPTIN_WAITING", "OPTIN_PIN_WAITING", "WAITING_FOR_CONFIRMATION":
			return true
		}
	}

	return false
}

// Confirm calls subscription-external confirm endpoint.
func (c *TIMWEClientImpl) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	c.logger.Info("Subscription confirm called",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID),
	)

	// partnerRoleID is intentionally ignored here - subscription-external owns TIMWE role selection.
	_ = partnerRoleID
	url := fmt.Sprintf("%s/api/external/v1/subscription/optin/confirm", strings.TrimRight(c.config.BaseURL, "/"))

	// Build request payload (TIMWE confirm requires MSISDN + productId + authCode)
	reqData := ConfirmRequest{
		UserIdentifier:      msisdn,
		UserIdentifierType:  "MSISDN",
		ProductID:           productID,
		MCC:                 c.config.MCC,
		MNC:                 c.config.MNC,
		EntryChannel:        entryChannel,
		TransactionAuthCode: authCode,
	}

	requestBody, err := json.Marshal(reqData)
	if err != nil {
		c.logger.Error("Failed to marshal confirm request", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal confirm request: %w", err)
	}

	// Log outbound request payload for debugging.
	var requestBodyMap map[string]interface{}
	_ = json.Unmarshal(requestBody, &requestBodyMap)
	c.logger.Info("Subscription confirm outbound request",
		zap.String("url", url),
		zap.String("method", "POST"),
		zap.Any("body", requestBodyMap),
		zap.String("raw_body", string(requestBody)),
	)

	// Execute with circuit breaker
	done, cbErr := c.circuitBreaker.Allow()
	if cbErr != nil {
		c.logger.Error("Circuit breaker is open", zap.Error(cbErr))
		return nil, fmt.Errorf("circuit breaker open: %w", cbErr)
	}

	// Execute request with retries.
	mtResp, err := c.sendMTRequestWithRetry(url, requestBody, outboundRequestMeta{
		Operation: "confirm",
		MSISDN:    msisdn,
		ProductID: productID,
	})
	if err != nil {
		done(false)
		return nil, err
	}
	done(true)

	// Prefer the inner subscription status from ResponseData over the top-level code,
	// since the top-level code may just indicate the API call succeeded (e.g. "SUCCESS")
	// while the inner status reflects the actual subscription state.
	confirmStatus := mtResp.Code
	if innerStatus, ok := extractConfirmResultStatus(mtResp); ok {
		confirmStatus = innerStatus
	}

	response := &TIMWEResponse{
		Success: c.isFinalConfirmSuccess(mtResp),
		Status:  confirmStatus,
		Message: sanitizeTIMWEMessage(mtResp.Message),
	}

	c.logger.Info("Subscription confirm response",
		zap.String("msisdn", msisdn),
		zap.String("code", mtResp.Code),
		zap.String("message", mtResp.Message),
		zap.Bool("in_error", mtResp.InError),
		zap.Any("response_data", mtResp.ResponseData),
		zap.Bool("success", response.Success),
	)

	return response, nil
}

// ConfirmWithDetails calls subscription-external confirm endpoint with fixed WEB channel.
func (c *TIMWEClientImpl) ConfirmWithDetails(msisdn string, productID int, authCode string) (*TIMWEResponse, error) {
	c.logger.Info("Subscription ConfirmWithDetails called",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID),
	)

	url := fmt.Sprintf("%s/api/external/v1/subscription/optin/confirm", strings.TrimRight(c.config.BaseURL, "/"))

	// Build request payload
	reqData := ConfirmRequest{
		UserIdentifier:      msisdn,
		UserIdentifierType:  "MSISDN",
		ProductID:           productID,
		MCC:                 c.config.MCC,
		MNC:                 c.config.MNC,
		EntryChannel:        "WEB",
		TransactionAuthCode: authCode,
	}

	requestBody, err := json.Marshal(reqData)
	if err != nil {
		c.logger.Error("Failed to marshal confirm request", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal confirm request: %w", err)
	}

	// Execute with circuit breaker
	done, cbErr := c.circuitBreaker.Allow()
	if cbErr != nil {
		c.logger.Error("Circuit breaker is open", zap.Error(cbErr))
		return nil, fmt.Errorf("circuit breaker open: %w", cbErr)
	}

	// Execute request with retries.
	mtResp, err := c.sendMTRequestWithRetry(url, requestBody, outboundRequestMeta{
		Operation: "confirm_with_details",
		MSISDN:    msisdn,
		ProductID: productID,
	})
	if err != nil {
		done(false)
		return nil, err
	}
	done(true)

	// Prefer inner subscription status from ResponseData over top-level code.
	confirmWithDetailsStatus := mtResp.Code
	if innerStatus, ok := extractConfirmResultStatus(mtResp); ok {
		confirmWithDetailsStatus = innerStatus
	}

	response := &TIMWEResponse{
		Success: c.isFinalConfirmSuccess(mtResp),
		Status:  confirmWithDetailsStatus,
		Message: sanitizeTIMWEMessage(mtResp.Message),
	}

	c.logger.Info("Subscription ConfirmWithDetails response",
		zap.String("msisdn", msisdn),
		zap.String("code", mtResp.Code),
		zap.Bool("success", response.Success),
	)

	return response, nil
}

// sendMTRequestWithRetry sends an outbound subscription request with exponential backoff retry.
func (c *TIMWEClientImpl) sendMTRequestWithRetry(url string, requestBody []byte, meta outboundRequestMeta) (*MTResponse, error) {
	baseDelay := c.config.RetryBaseDelay
	maxRetries := c.config.MaxRetries
	maxDelay := c.config.RetryMaxDelay
	seenExternalTxIDs := make(map[string]struct{}, maxRetries)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		externalTxID := uuid.New().String()
		for {
			if _, exists := seenExternalTxIDs[externalTxID]; !exists {
				break
			}
			externalTxID = uuid.New().String()
		}
		seenExternalTxIDs[externalTxID] = struct{}{}

		// Set up request
		req.SetRequestURI(url)
		req.Header.SetMethod("POST")
		req.Header.Set("external-tx-id", externalTxID)
		req.Header.Set("Content-Type", "application/json")
		for key, value := range meta.Headers {
			req.Header.Set(key, value)
		}
		req.SetBody(requestBody)

		c.logger.Info("Sending outbound TIMWE request",
			zap.String("operation", meta.Operation),
			zap.Int("attempt", attempt),
			zap.String("external_tx_id", externalTxID),
			zap.String("url", url),
			zap.Int("product_id", meta.ProductID),
			zap.String("msisdn_prefix", msisdnPrefix(meta.MSISDN)),
		)

		// Execute request
		err := c.client.Do(req, res)

		if err != nil {
			c.logger.Warn("Failed to send request",
				zap.Int("attempt", attempt),
				zap.String("url", url),
				zap.Error(err))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to send request after %d attempts: %w", maxRetries, err)
			}

			// Exponential backoff
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
			continue
		}

		// Check HTTP status
		statusCode := res.StatusCode()
		if statusCode != fasthttp.StatusOK {
			responseBody := string(res.Body())
			errorDetails := extractUpstreamErrorDetails(res.Body())

			c.logger.Error("Request failed with non-200 status",
				zap.Int("attempt", attempt),
				zap.Int("status_code", statusCode),
				zap.String("response_body", responseBody),
				zap.String("url", url))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			// Don't retry on client errors (4xx)
			if statusCode >= 400 && statusCode < 500 {
				if errorDetails != "" {
					return nil, fmt.Errorf("request failed with status code: %d (%s)", statusCode, errorDetails)
				}
				return nil, fmt.Errorf("request failed with status code: %d", statusCode)
			}

			if attempt == maxRetries {
				if errorDetails != "" {
					return nil, fmt.Errorf("request failed with status code %d after %d attempts (%s)", statusCode, maxRetries, errorDetails)
				}
				return nil, fmt.Errorf("request failed with status code %d after %d attempts", statusCode, maxRetries)
			}

			// Exponential backoff for server errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
			continue
		}

		// Parse response
		var mtResponse MTResponse
		if err := json.Unmarshal(res.Body(), &mtResponse); err != nil {
			c.logger.Error("Failed to parse response",
				zap.Int("attempt", attempt),
				zap.String("response_body", string(res.Body())),
				zap.Error(err))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to parse response after %d attempts: %w", maxRetries, err)
			}

			// Exponential backoff
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
			continue
		}

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)

		// Check for INTERNAL_ERROR which should trigger retry
		if mtResponse.Code == "INTERNAL_ERROR" && attempt < maxRetries {
			c.logger.Warn("TIMWE returned INTERNAL_ERROR, retrying",
				zap.Int("attempt", attempt),
				zap.String("url", url))

			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}
			time.Sleep(delay)
			continue
		}

		return &mtResponse, nil
	}

	return nil, fmt.Errorf("exhausted all retry attempts")
}

func (c *TIMWEClientImpl) signedTenantHeaders(method, path string, body []byte, tenant TenantSubscriptionContext) (map[string]string, error) {
	secret := strings.TrimSpace(c.config.TrustedServiceSecret)
	if secret == "" {
		return nil, fmt.Errorf("trusted service secret is required for tenant subscription routing")
	}
	serviceID := strings.TrimSpace(c.config.ServiceID)
	if serviceID == "" {
		serviceID = "acquisition-api"
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	bodySHA := tenantctx.BodySHA256(body)
	signature := tenantctx.SignServiceRequest(secret, tenantctx.SignInput{
		Method:    method,
		Path:      path,
		Timestamp: timestamp,
		Nonce:     nonce,
		ServiceID: serviceID,
		TenantID:  tenant.TenantID,
		BodySHA:   bodySHA,
	})
	return map[string]string{
		tenantctx.HeaderTenantID:         tenant.TenantID,
		tenantctx.HeaderServiceID:        serviceID,
		tenantctx.HeaderServiceTimestamp: timestamp,
		tenantctx.HeaderServiceNonce:     nonce,
		tenantctx.HeaderServiceBodySHA:   bodySHA,
		tenantctx.HeaderServiceSignature: signature,
		"X-Tenant-Channel-Id":            tenant.ChannelID,
	}, nil
}

func msisdnPrefix(msisdn string) string {
	if msisdn == "" {
		return ""
	}
	prefixLen := 5
	if len(msisdn) < prefixLen {
		prefixLen = len(msisdn)
	}
	return msisdn[:prefixLen]
}

func extractUpstreamErrorDetails(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	parts := make([]string, 0, 2)
	if code := strings.TrimSpace(payload.Code); code != "" {
		parts = append(parts, "code="+code)
	}

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = strings.TrimSpace(payload.Error)
	}
	if message != "" {
		parts = append(parts, "message="+message)
	}

	return strings.Join(parts, " ")
}

func (c *TIMWEClientImpl) isFinalConfirmSuccess(mtResp *MTResponse) bool {
	if mtResp == nil || mtResp.InError {
		return false
	}

	if status, ok := extractConfirmResultStatus(mtResp); ok {
		c.logger.Debug("Confirm inner status extracted",
			zap.String("inner_status", status),
			zap.String("top_level_code", mtResp.Code),
			zap.String("message", mtResp.Message),
		)
		switch status {
		case "OPTIN_PREACTIVE_WAIT_CONF", "OPTIN_WAITING", "OPTIN_PIN_WAITING", "WAITING_FOR_CONFIRMATION":
			return false
		case "SUBSCRIBED", "CONFIRMED", "OPTIN_SUCCESS", "SUCCESS":
			return true
		}
	}

	switch strings.ToUpper(strings.TrimSpace(mtResp.Code)) {
	case "SUBSCRIBED", "CONFIRMED", "OPTIN_SUCCESS":
		return true
	case "SUCCESS":
		// Some partners return SUCCESS without a terminal subscriptionResult.
		// Treat it as final unless the message explicitly indicates pending state.
		return !confirmMessageIndicatesPending(mtResp.Message)
	}

	return false
}

func extractConfirmResultStatus(mtResp *MTResponse) (string, bool) {
	if mtResp == nil || mtResp.ResponseData == nil {
		return "", false
	}

	for _, key := range []string{"subscriptionResult", "status", "subscriptionStatus"} {
		raw, ok := mtResp.ResponseData[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			continue
		}
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		return normalized, true
	}

	return "", false
}

func confirmMessageIndicatesPending(raw string) bool {
	message := strings.ToLower(strings.TrimSpace(raw))
	if message == "" {
		return false
	}

	for _, keyword := range []string{"pending", "wait", "waiting", "not finalized", "processing"} {
		if strings.Contains(message, keyword) {
			return true
		}
	}

	return false
}

func sanitizeTIMWEMessage(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "null") || strings.EqualFold(trimmed, "nil") {
		return ""
	}
	return trimmed
}
