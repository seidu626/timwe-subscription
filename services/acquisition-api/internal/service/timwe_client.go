package service

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/utils"
	"github.com/sony/gobreaker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// TIMWEConfig holds configuration for TIMWE API client
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
	CBMaxRequests            uint32
	CBTimeout                time.Duration
	CBInterval               time.Duration
	CBMinRequests            uint32
	CBFailureRateThreshold   float64
	CBConsecutiveFailures    uint32
}

// DefaultTIMWEConfig returns default TIMWE configuration
func DefaultTIMWEConfig() *TIMWEConfig {
	return &TIMWEConfig{
		BaseURL:                "https://tigo.timwe.com/gh/ma/api/external/v1",
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

// MTRequest represents the TIMWE MT (Mobile Terminated) request payload
type MTRequest struct {
	ProductID          int    `json:"productId"`
	PricepointID       int    `json:"pricepointId,omitempty"`
	MCC                string `json:"mcc"`
	MNC                string `json:"mnc"`
	UserIdentifier     string `json:"userIdentifier"`
	UserIdentifierType string `json:"userIdentifierType"`
	EntryChannel       string `json:"entryChannel"`
	SubKeyword         string `json:"subKeyword,omitempty"`
	LargeAccount       string `json:"largeAccount,omitempty"`
	CampaignUrl        string `json:"campaignUrl,omitempty"`
	SendDate           string `json:"sendDate,omitempty"`
	Priority           string `json:"priority,omitempty"`
	Timezone           string `json:"timezone,omitempty"`
	Context            string `json:"context,omitempty"`
	MoTransactionUUID  string `json:"moTransactionUUID"`
}

// MTResponse represents the TIMWE MT response
type MTResponse struct {
	ResponseData map[string]interface{} `json:"responseData"`
	Message      string                 `json:"message"`
	InError      bool                   `json:"inError"`
	RequestID    string                 `json:"requestId"`
	Code         string                 `json:"code"`
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
		MaxConnsPerHost:          config.MaxConnections,
		MaxIdleConnDuration:      config.IdleConnTimeout,
		ReadTimeout:              config.Timeout,
		WriteTimeout:             config.Timeout,
		MaxConnWaitTimeout:       config.DialTimeout,
		MaxResponseBodySize:      10 * 1024 * 1024, // 10MB
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

// OptIn calls TIMWE opt-in API
// partnerRoleID overrides config.PartnerRoleID when provided (campaign-specific).
func (c *TIMWEClientImpl) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	c.logger.Info("TIMWE OptIn called",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID),
		zap.String("entry_channel", entryChannel),
	)

	// Generate authentication key
	authKey, err := utils.GetCachedAuthKey(c.config.PartnerServiceID, c.config.PSK)
	if err != nil {
		c.logger.Error("Failed to generate auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to generate auth key: %w", err)
	}

	// Generate transaction UUID
	txUUID := uuid.New().String()

	// Build request payload
	reqData := MTRequest{
		ProductID:          productID,
		MCC:                c.config.MCC,
		MNC:                c.config.MNC,
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		EntryChannel:       entryChannel,
		MoTransactionUUID:  txUUID,
	}

	// Add tracking fields as context
	if len(trackingFields) > 0 {
		contextData, _ := json.Marshal(trackingFields)
		reqData.Context = string(contextData)
	}

	// Build URL (campaign-specific partner role when provided)
	role := c.config.PartnerRoleID
	if partnerRoleID != "" {
		role = partnerRoleID
	}
	url := fmt.Sprintf("%s/subscription/optin/%s", c.config.BaseURL, role)

	// Marshal request body
	requestBody, err := json.Marshal(reqData)
	if err != nil {
		c.logger.Error("Failed to marshal request data", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	// Execute with circuit breaker
	done, cbErr := c.circuitBreaker.Allow()
	if cbErr != nil {
		c.logger.Error("Circuit breaker is open", zap.Error(cbErr))
		return nil, fmt.Errorf("circuit breaker open: %w", cbErr)
	}

	// Execute request with retries
	mtResp, err := c.sendMTRequestWithRetry(url, authKey, txUUID, requestBody)
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
		Message:       mtResp.Message,
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

	c.logger.Info("TIMWE OptIn response",
		zap.String("msisdn", msisdn),
		zap.String("code", mtResp.Code),
		zap.Bool("success", response.Success),
		zap.Bool("requires_confirm", response.RequiresConfirm),
	)

	return response, nil
}

// Confirm calls TIMWE confirm API with full transaction details.
// partnerRoleID overrides config.PartnerRoleID when provided (campaign-specific).
func (c *TIMWEClientImpl) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	c.logger.Info("TIMWE Confirm called",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID),
	)

	// Generate authentication key
	authKey, err := utils.GetCachedAuthKey(c.config.PartnerServiceID, c.config.PSK)
	if err != nil {
		c.logger.Error("Failed to generate auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to generate auth key: %w", err)
	}

	// Build URL (campaign-specific partner role when provided)
	role := c.config.PartnerRoleID
	if partnerRoleID != "" {
		role = partnerRoleID
	}
	url := fmt.Sprintf("%s/subscription/optin/confirm/%s", c.config.BaseURL, role)

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

	// Generate external transaction ID
	externalTxID := uuid.New().String()

	// Execute with circuit breaker
	done, cbErr := c.circuitBreaker.Allow()
	if cbErr != nil {
		c.logger.Error("Circuit breaker is open", zap.Error(cbErr))
		return nil, fmt.Errorf("circuit breaker open: %w", cbErr)
	}

	// Execute request with retries
	mtResp, err := c.sendMTRequestWithRetry(url, authKey, externalTxID, requestBody)
	if err != nil {
		done(false)
		return nil, err
	}
	done(true)

	// Convert to TIMWEResponse
	response := &TIMWEResponse{
		Success: !mtResp.InError,
		Status:        mtResp.Code,
		Message:       mtResp.Message,
	}

	// Check success codes
	switch mtResp.Code {
	case "SUBSCRIBED", "SUCCESS", "OPTIN_SUCCESS", "CONFIRMED":
		response.Success = true
	default:
		if mtResp.InError {
			response.Success = false
		}
	}

	c.logger.Info("TIMWE Confirm response",
		zap.String("msisdn", msisdn),
		zap.String("code", mtResp.Code),
		zap.Bool("success", response.Success),
	)

	return response, nil
}

// ConfirmWithDetails calls TIMWE confirm API with full transaction details
func (c *TIMWEClientImpl) ConfirmWithDetails(msisdn string, productID int, authCode string) (*TIMWEResponse, error) {
	c.logger.Info("TIMWE ConfirmWithDetails called",
		zap.String("msisdn", msisdn),
		zap.Int("product_id", productID),
	)

	// Generate authentication key
	authKeyValue, err := utils.GetCachedAuthKey(c.config.PartnerServiceID, c.config.PSK)
	if err != nil {
		c.logger.Error("Failed to generate auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to generate auth key: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s/subscription/optin/confirm/%s", c.config.BaseURL, c.config.PartnerRoleID)

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

	// Generate external transaction ID
	externalTxID := uuid.New().String()

	// Execute with circuit breaker
	done, cbErr := c.circuitBreaker.Allow()
	if cbErr != nil {
		c.logger.Error("Circuit breaker is open", zap.Error(cbErr))
		return nil, fmt.Errorf("circuit breaker open: %w", cbErr)
	}

	// Execute request with retries
	mtResp, err := c.sendMTRequestWithRetry(url, authKeyValue, externalTxID, requestBody)
	if err != nil {
		done(false)
		return nil, err
	}
	done(true)

	// Convert to TIMWEResponse
	response := &TIMWEResponse{
		Success: !mtResp.InError,
		Status:  mtResp.Code,
		Message: mtResp.Message,
	}

	// Check success codes
	switch mtResp.Code {
	case "SUBSCRIBED", "SUCCESS", "OPTIN_SUCCESS", "CONFIRMED":
		response.Success = true
	default:
		if mtResp.InError {
			response.Success = false
		}
	}

	c.logger.Info("TIMWE ConfirmWithDetails response",
		zap.String("msisdn", msisdn),
		zap.String("code", mtResp.Code),
		zap.Bool("success", response.Success),
	)

	return response, nil
}

// sendMTRequestWithRetry sends an MT request with exponential backoff retry
func (c *TIMWEClientImpl) sendMTRequestWithRetry(url, authKey, externalTxID string, requestBody []byte) (*MTResponse, error) {
	baseDelay := c.config.RetryBaseDelay
	maxRetries := c.config.MaxRetries
	maxDelay := c.config.RetryMaxDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()

		// Set up request
		req.SetRequestURI(url)
		req.Header.SetMethod("POST")
		req.Header.Set("apikey", c.config.APIKey)
		req.Header.Set("authentication", authKey)
		req.Header.Set("external-tx-id", externalTxID)
		req.Header.Set("Content-Type", "application/json")
		req.SetBody(requestBody)

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
			c.logger.Error("Request failed with non-200 status",
				zap.Int("attempt", attempt),
				zap.Int("status_code", statusCode),
				zap.String("response_body", string(res.Body())),
				zap.String("url", url))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			// Don't retry on client errors (4xx)
			if statusCode >= 400 && statusCode < 500 {
				return nil, fmt.Errorf("request failed with status code: %d", statusCode)
			}

			if attempt == maxRetries {
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
