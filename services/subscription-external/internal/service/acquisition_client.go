package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// AcquisitionClient handles internal calls to the acquisition-api service
type AcquisitionClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
	logger     *zap.Logger
	enabled    bool
}

// ChargeSuccessRequest represents the payload for charge-success notification
type ChargeSuccessRequest struct {
	TimweTransactionID string `json:"timwe_transaction_id"`
	MSISDN             string `json:"msisdn,omitempty"`
	ProductID          int    `json:"product_id,omitempty"`
	ChargedAt          string `json:"charged_at,omitempty"`
	Payout             string `json:"payout,omitempty"`
}

// ChargeSuccessResponse represents the response from charge-success endpoint
type ChargeSuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NewAcquisitionClient creates a new acquisition API client
func NewAcquisitionClient(logger *zap.Logger) *AcquisitionClient {
	// Get configuration from environment
	baseURL := os.Getenv("ACQUISITION_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8084" // Default for development
	}

	secret := os.Getenv("INTERNAL_API_SECRET")
	if secret == "" {
		secret = "dev-internal-secret-change-in-production"
		logger.Warn("INTERNAL_API_SECRET not set, using development default - DO NOT USE IN PRODUCTION")
	}

	// Check if charge callback is enabled (default: enabled)
	enabled := true
	if v := os.Getenv("ACQUISITION_CHARGE_CALLBACK_ENABLED"); v == "false" || v == "0" {
		enabled = false
		logger.Info("Acquisition charge callback is disabled")
	}

	return &AcquisitionClient{
		baseURL: baseURL,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger,
		enabled: enabled,
	}
}

// NotifyChargeSuccess calls acquisition-api to notify of a successful charge
// This is used to trigger conversion postbacks (e.g., for Mobplus)
func (c *AcquisitionClient) NotifyChargeSuccess(req *ChargeSuccessRequest) error {
	if !c.enabled {
		c.logger.Debug("Charge callback disabled, skipping notification",
			zap.String("timwe_transaction_id", req.TimweTransactionID))
		return nil
	}

	if req.TimweTransactionID == "" {
		return fmt.Errorf("timwe_transaction_id is required")
	}

	// Serialize request body
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create timestamp for signature
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Generate HMAC signature: HMAC-SHA256(secret, timestamp + body)
	message := timestamp + string(body)
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	// Build request
	url := fmt.Sprintf("%s/internal/acquisition/charge-success", c.baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Timestamp", timestamp)
	httpReq.Header.Set("X-Internal-Signature", signature)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("Failed to call acquisition-api charge-success",
			zap.String("timwe_transaction_id", req.TimweTransactionID),
			zap.Error(err))
		return fmt.Errorf("failed to call acquisition-api: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, _ := io.ReadAll(resp.Body)

	// Check status
	if resp.StatusCode == http.StatusNotFound {
		// Transaction not found in acquisition-api - this is normal for non-web acquisitions
		c.logger.Debug("Transaction not found in acquisition-api (likely not a web acquisition)",
			zap.String("timwe_transaction_id", req.TimweTransactionID))
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Acquisition-api charge-success returned error",
			zap.String("timwe_transaction_id", req.TimweTransactionID),
			zap.Int("status", resp.StatusCode),
			zap.String("response", string(respBody)))
		return fmt.Errorf("acquisition-api returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.logger.Info("Charge success notification sent to acquisition-api",
		zap.String("timwe_transaction_id", req.TimweTransactionID),
		zap.String("msisdn", req.MSISDN))

	return nil
}

// NotifyChargeSuccessAsync calls NotifyChargeSuccess in a goroutine
// Use this when you don't want to block on the HTTP call
func (c *AcquisitionClient) NotifyChargeSuccessAsync(req *ChargeSuccessRequest) {
	go func() {
		if err := c.NotifyChargeSuccess(req); err != nil {
			c.logger.Error("Async charge success notification failed",
				zap.String("timwe_transaction_id", req.TimweTransactionID),
				zap.Error(err))
		}
	}()
}
