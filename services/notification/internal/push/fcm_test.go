package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

type staticTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s staticTokenSource) Token() (*oauth2.Token, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.token, nil
}

func newTestSender(t *testing.T, srv *httptest.Server) *FCMSender {
	t.Helper()
	return &FCMSender{
		projectID:  "test-project",
		tokenSrc:   staticTokenSource{token: &oauth2.Token{AccessToken: "test-access-token"}},
		httpClient: srv.Client(),
		endpoint:   srv.URL,
	}
}

func TestSend_PostsExpectedPayloadAndAuth(t *testing.T) {
	var gotAuth string
	var gotBody fcmRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"projects/test-project/messages/0"}`))
	}))
	defer srv.Close()

	sender := newTestSender(t, srv)
	if err := sender.Send(context.Background(), "device-token-abc", "Hello from Dayline"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotAuth != "Bearer test-access-token" {
		t.Errorf("Authorization = %q, want Bearer test-access-token", gotAuth)
	}
	if gotBody.Message.Token != "device-token-abc" {
		t.Errorf("token = %q, want device-token-abc", gotBody.Message.Token)
	}
	if gotBody.Message.Notification == nil || gotBody.Message.Notification.Body != "Hello from Dayline" {
		t.Errorf("unexpected notification: %+v", gotBody.Message.Notification)
	}
}

func TestSend_ReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"not registered"}}`))
	}))
	defer srv.Close()

	sender := newTestSender(t, srv)
	if err := sender.Send(context.Background(), "device-token-abc", "body"); err == nil {
		t.Fatal("expected error for non-2xx FCM response")
	}
}

func TestSend_ReturnsErrorOnTokenFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when token retrieval fails")
	}))
	defer srv.Close()

	sender := &FCMSender{
		projectID:  "test-project",
		tokenSrc:   staticTokenSource{err: context.DeadlineExceeded},
		httpClient: srv.Client(),
		endpoint:   srv.URL,
	}
	if err := sender.Send(context.Background(), "device-token-abc", "body"); err == nil {
		t.Fatal("expected error when oauth token retrieval fails")
	}
}

func TestSend_ReturnsErrorOnMissingDeviceToken(t *testing.T) {
	sender := &FCMSender{
		projectID:  "test-project",
		tokenSrc:   staticTokenSource{token: &oauth2.Token{AccessToken: "x"}},
		httpClient: http.DefaultClient,
		endpoint:   "http://unused.invalid",
	}
	if err := sender.Send(context.Background(), "", "body"); err == nil {
		t.Fatal("expected error for missing device token")
	}
}

func TestSend_NilSenderFailsClosed(t *testing.T) {
	var sender *FCMSender
	if err := sender.Send(context.Background(), "token", "body"); err == nil {
		t.Fatal("expected error from nil sender")
	}
}

func TestNewFCMSenderFromEnv_ReturnsNilWhenUnset(t *testing.T) {
	t.Setenv(EnvCredentialsPath, "")
	sender, err := NewFCMSenderFromEnv(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender != nil {
		t.Fatal("expected nil sender when FCM_CREDENTIALS_JSON_PATH is unset")
	}
}

func TestNewFCMSender_RejectsMissingProjectID(t *testing.T) {
	_, err := NewFCMSender(context.Background(), []byte(`{"type":"service_account"}`))
	if err == nil {
		t.Fatal("expected error for credentials missing project_id")
	}
}
