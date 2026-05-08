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

// AcquisitionClient handles internal calls to acquisition-api.
type AcquisitionClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
	logger     *zap.Logger
	enabled    bool
}

// ChargeSuccessRequest represents the payload for charge-success notification.
type ChargeSuccessRequest struct {
	TimweTransactionID string `json:"timwe_transaction_id"`
	MSISDN             string `json:"msisdn,omitempty"`
	ProductID          int    `json:"product_id,omitempty"`
	ChargedAt          string `json:"charged_at,omitempty"`
	Payout             string `json:"payout,omitempty"`
}

// NewAcquisitionClient creates a new acquisition API client.
func NewAcquisitionClient(logger *zap.Logger) *AcquisitionClient {
	baseURL := os.Getenv("ACQUISITION_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8084"
	}

	secret := os.Getenv("INTERNAL_API_SECRET")
	if secret == "" {
		secret = "dev-internal-secret-change-in-production"
		logger.Warn("INTERNAL_API_SECRET not set, using development default - DO NOT USE IN PRODUCTION")
	}

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

// NotifyChargeSuccess calls acquisition-api to notify of a successful charge.
func (c *AcquisitionClient) NotifyChargeSuccess(req *ChargeSuccessRequest) error {
	if !c.enabled {
		return nil
	}
	if req.TimweTransactionID == "" {
		return fmt.Errorf("timwe_transaction_id is required")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	message := timestamp + string(body)
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	url := fmt.Sprintf("%s/internal/acquisition/charge-success", c.baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Timestamp", timestamp)
	httpReq.Header.Set("X-Internal-Signature", signature)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to call acquisition-api: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		c.logger.Debug("Transaction not found in acquisition-api (likely not a web acquisition)",
			zap.String("timwe_transaction_id", req.TimweTransactionID))
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("acquisition-api returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// NotifyChargeSuccessAsync calls NotifyChargeSuccess in a goroutine.
func (c *AcquisitionClient) NotifyChargeSuccessAsync(req *ChargeSuccessRequest) {
	go func() {
		if err := c.NotifyChargeSuccess(req); err != nil {
			c.logger.Error("Async charge success notification failed",
				zap.String("timwe_transaction_id", req.TimweTransactionID),
				zap.Error(err))
		}
	}()
}
