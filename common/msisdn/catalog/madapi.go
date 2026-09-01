package catalog

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrMADAPINotConfigured = errors.New("MADAPI is not configured")

type MADAPIConfig struct {
	BaseURL      string
	TokenURL     string
	KYCPath      string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
	MaxRetries   int
}

func MADAPIConfigFromEnv() MADAPIConfig {
	timeout := 15 * time.Second
	if parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv("MADAPI_TIMEOUT"))); err == nil && parsed > 0 {
		timeout = parsed
	}
	retries := 3
	if parsed, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MADAPI_MAX_RETRIES"))); err == nil && parsed >= 0 && parsed <= 5 {
		retries = parsed
	}
	return MADAPIConfig{
		BaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("MADAPI_BASE_URL")), "/"),
		TokenURL:     strings.TrimSpace(os.Getenv("MADAPI_TOKEN_URL")),
		KYCPath:      strings.TrimSpace(os.Getenv("MADAPI_KYC_PATH")),
		ClientID:     strings.TrimSpace(os.Getenv("MADAPI_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("MADAPI_CLIENT_SECRET")),
		Timeout:      timeout,
		MaxRetries:   retries,
	}
}

type MADAPIClient struct {
	config      MADAPIConfig
	client      *http.Client
	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func NewMADAPIClient(config MADAPIConfig, client *http.Client) *MADAPIClient {
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	if config.KYCPath == "" {
		config.KYCPath = "/kyc/{customerId}"
	}
	return &MADAPIClient{config: config, client: client}
}

func (c *MADAPIClient) Configured() bool {
	return c != nil && c.config.BaseURL != "" && c.config.TokenURL != "" && c.config.ClientID != "" && c.config.ClientSecret != ""
}

func (c *MADAPIClient) Verify(ctx context.Context, msisdn string) VerificationResult {
	result := VerificationResult{Provider: "MADAPI"}
	if !c.Configured() {
		result.Status, result.Reason = StatusError, ErrMADAPINotConfigured.Error()
		return result
	}
	token, err := c.accessToken(ctx, false)
	if err != nil {
		result.Status, result.Reason, result.Retryable = StatusError, err.Error(), true
		return result
	}
	status, reference, retryable, err := c.lookup(ctx, token, msisdn)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		token, err = c.accessToken(ctx, true)
		if err == nil {
			status, reference, retryable, err = c.lookup(ctx, token, msisdn)
		}
	}
	result.Reference = reference
	if err != nil {
		result.Status, result.Reason, result.Retryable = StatusError, err.Error(), retryable
		return result
	}
	switch {
	case status == http.StatusNotFound:
		result.Status, result.Reason = StatusInvalid, "MADAPI has no KYC record"
	case status >= 200 && status < 300:
		result.Status = StatusVerified
	default:
		result.Status = StatusError
		result.Reason = fmt.Sprintf("MADAPI returned HTTP %d", status)
		result.Retryable = status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
	}
	return result
}

func (c *MADAPIClient) accessToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && time.Now().Add(5*time.Minute).Before(c.tokenExpiry) {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create MADAPI token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.config.ClientID+":"+c.config.ClientSecret)))
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request MADAPI token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("MADAPI token endpoint returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil || payload.AccessToken == "" {
		return "", fmt.Errorf("MADAPI token response was malformed")
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 1800
	}
	c.token, c.tokenExpiry = payload.AccessToken, time.Now().Add(time.Duration(payload.ExpiresIn)*time.Second)
	return c.token, nil
}

func (c *MADAPIClient) lookup(ctx context.Context, token, msisdn string) (int, string, bool, error) {
	path := strings.ReplaceAll(c.config.KYCPath, "{customerId}", url.PathEscape(msisdn))
	fullURL := c.config.BaseURL + "/" + strings.TrimLeft(path, "/")
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, "", true, ctx.Err()
			case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return 0, "", false, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("transactionId", fmt.Sprintf("catalog-%d", time.Now().UnixNano()))
		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("MADAPI returned HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp.StatusCode, "", false, nil
		}
		var payload struct {
			TransactionID string          `json:"transactionId"`
			Data          json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return resp.StatusCode, "", false, fmt.Errorf("MADAPI KYC response was malformed")
		}
		if len(payload.Data) == 0 || string(payload.Data) == "null" || string(payload.Data) == "{}" {
			return resp.StatusCode, payload.TransactionID, false, fmt.Errorf("MADAPI returned an empty KYC payload")
		}
		return resp.StatusCode, payload.TransactionID, false, nil
	}
	return 0, "", true, fmt.Errorf("MADAPI request failed after retries: %w", lastErr)
}
