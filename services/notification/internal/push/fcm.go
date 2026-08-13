// Package push implements PUSH notification delivery for the Dayline app via
// the FCM (Firebase Cloud Messaging) HTTP v1 API.
// See docs/dayline-app-api-contract.md "Push delivery".
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// EnvCredentialsPath is the environment variable holding the path to the FCM
// service-account JSON credential file.
const EnvCredentialsPath = "FCM_CREDENTIALS_JSON_PATH"

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// Sender sends a push notification body to a single registered device.
type Sender interface {
	Send(ctx context.Context, deviceToken, body string) error
}

// tokenSource is the subset of oauth2.TokenSource FCMSender depends on, so
// tests can inject a fake without a real Google credential.
type tokenSource interface {
	Token() (*oauth2.Token, error)
}

// FCMSender sends push notifications via the FCM HTTP v1 API using an
// OAuth2 service-account credential.
type FCMSender struct {
	projectID  string
	tokenSrc   tokenSource
	httpClient *http.Client
	endpoint   string
}

// NewFCMSenderFromEnv builds an FCMSender from the service-account JSON at
// FCM_CREDENTIALS_JSON_PATH. It returns (nil, nil) when the env var is
// unset: callers MUST treat a nil sender as "PUSH not configured" and fall
// back to SMS so content is never dropped, per the contract.
func NewFCMSenderFromEnv(ctx context.Context) (*FCMSender, error) {
	path := strings.TrimSpace(os.Getenv(EnvCredentialsPath))
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read FCM credentials from %s: %w", path, err)
	}
	return NewFCMSender(ctx, raw)
}

// NewFCMSender builds an FCMSender from raw service-account JSON bytes.
func NewFCMSender(ctx context.Context, credentialsJSON []byte) (*FCMSender, error) {
	var parsed struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(credentialsJSON, &parsed); err != nil {
		return nil, fmt.Errorf("parse FCM credentials: %w", err)
	}
	if strings.TrimSpace(parsed.ProjectID) == "" {
		return nil, fmt.Errorf("FCM credentials missing project_id")
	}
	creds, err := google.CredentialsFromJSON(ctx, credentialsJSON, fcmScope)
	if err != nil {
		return nil, fmt.Errorf("build FCM credentials: %w", err)
	}
	return &FCMSender{
		projectID:  parsed.ProjectID,
		tokenSrc:   creds.TokenSource,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		endpoint:   fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", parsed.ProjectID),
	}, nil
}

type fcmRequest struct {
	Message fcmMessage `json:"message"`
}

type fcmMessage struct {
	Token        string           `json:"token"`
	Notification *fcmNotification `json:"notification,omitempty"`
}

type fcmNotification struct {
	Body string `json:"body"`
}

// Send delivers body to deviceToken via the FCM HTTP v1 API.
func (s *FCMSender) Send(ctx context.Context, deviceToken, body string) error {
	if s == nil || s.tokenSrc == nil {
		return fmt.Errorf("fcm sender not configured")
	}
	if strings.TrimSpace(deviceToken) == "" {
		return fmt.Errorf("fcm send: missing device token")
	}

	token, err := s.tokenSrc.Token()
	if err != nil {
		return fmt.Errorf("fcm oauth token: %w", err)
	}

	payload := fcmRequest{Message: fcmMessage{
		Token:        deviceToken,
		Notification: &fcmNotification{Body: body},
	}}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal fcm payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build fcm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fcm status %d", resp.StatusCode)
	}
	return nil
}
