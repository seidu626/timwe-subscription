package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/common/tenantevents"
)

const (
	optoutTestTenant = "bc6fa5c6-e54f-4220-bf4b-6ef1d4716d8d"
	optoutTestEvent  = "evt-1"
	optoutTestEnv    = "TEST_TENANT_EVENTS_BINDING"
)

// bindTestTenantEvents points the tenant's binding at webhookURL through an env:// secret reference.
func bindTestTenantEvents(t *testing.T, webhookURL string, honor bool) {
	t.Helper()
	t.Setenv(tenantevents.AllowPrivateTargetsEnv, "1")
	blob, _ := json.Marshal(map[string]any{
		"url": webhookURL, "signing_secret": "0123456789abcdef", "default_amount": 0.8,
		"default_currency": "GHS", "honor_optout_action": honor,
	})
	t.Setenv(optoutTestEnv, string(blob))
	previous := tenantWebhookHTTPClient
	tenantWebhookHTTPClient = newTenantWebhookHTTPClient(true)
	t.Cleanup(func() { tenantWebhookHTTPClient = previous })
}

func expectActiveBinding(mock sqlmock.Sqlmock, bound bool) {
	rows := sqlmock.NewRows([]string{"channel_id", "secret_ref"})
	if bound {
		rows.AddRow("42a02ebc-c2dc-43ce-94de-d552a87a74ef", "env://"+optoutTestEnv)
	}
	mock.ExpectQuery(regexp.QuoteMeta(activeTenantEventBindingSQL)).WithArgs(optoutTestTenant).WillReturnRows(rows)
}

func expectOptoutClaim(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`WITH due AS .* SET optout_status = 'sending'`).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "tenant_id", "optout_attempts"}).
			AddRow(optoutTestEvent, optoutTestTenant, 1))
}

// stubSubext counts opt-out calls the dispatcher makes to subscription-external.
func stubSubext(t *testing.T) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(srv.Close)
	t.Setenv("SUBSCRIPTION_EXTERNAL_BASE_URL", srv.URL)
	t.Setenv("GATEWAY_TRUST_SECRET", "test-trust-secret")
	return &calls
}

func TestDispatchTenantEvent_OptoutReplyQueuesOnlyWhenHonored(t *testing.T) {
	for _, honor := range []bool{false, true} {
		t.Run(map[bool]string{false: "default off", true: "explicitly on"}[honor], func(t *testing.T) {
			reply := `{"ok":true,"outcome":"held","reason":"NO_SUBSCRIPTION","action":"optout"}`
			webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(reply))
			}))
			defer webhook.Close()
			bindTestTenantEvents(t, webhook.URL, honor)
			repo, mock := newNotificationRepoWithMock(t)
			mock.ExpectQuery(`WITH due AS .* UPDATE tenant_event_outbox o\s+SET status = 'sending'`).
				WillReturnRows(sqlmock.NewRows([]string{"event_id", "tenant_id", "payload", "attempts"}).
					AddRow(optoutTestEvent, optoutTestTenant, []byte(`{"type":"RENEWAL_SUCCESS","msisdn":"233260000000"}`), 1))
			expectActiveBinding(mock, true)
			mock.ExpectExec(`UPDATE tenant_event_outbox\s+SET status = \$3`).
				WithArgs(optoutTestTenant, optoutTestEvent, "delivered", sqlmock.AnyArg(), 200, "", reply, honor).
				WillReturnResult(sqlmock.NewResult(0, 1))

			claimed, err := repo.dispatchOneTenantEvent(context.Background())
			if err != nil || !claimed {
				t.Fatalf("dispatchOneTenantEvent = %v, %v; want claimed, nil", claimed, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDispatchTenantOptout_ClosesWithoutCallingProviderUnlessHonored(t *testing.T) {
	for name, bound := range map[string]bool{"flag off": true, "no binding": false} {
		t.Run(name, func(t *testing.T) {
			calls := stubSubext(t)
			bindTestTenantEvents(t, "http://127.0.0.1:1/events", false)
			repo, mock := newNotificationRepoWithMock(t)
			expectOptoutClaim(mock)
			expectActiveBinding(mock, bound)
			mock.ExpectExec(`UPDATE tenant_event_outbox\s+SET optout_status = \$3`).
				WithArgs(optoutTestTenant, optoutTestEvent, "failed", sqlmock.AnyArg(), tenantOptoutDisabled).
				WillReturnResult(sqlmock.NewResult(0, 1))

			if err := repo.dispatchOneTenantOptout(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			if n := calls.Load(); n != 0 {
				t.Fatalf("subscription-external opt-out called %d times; want 0", n)
			}
		})
	}
}

func TestDispatchTenantOptout_SendsWhenHonored(t *testing.T) {
	calls := stubSubext(t)
	bindTestTenantEvents(t, "http://127.0.0.1:1/events", true)
	repo, mock := newNotificationRepoWithMock(t)
	expectOptoutClaim(mock)
	expectActiveBinding(mock, true)
	mock.ExpectQuery(`SELECT t.tenant_key, c.channel_key, n.msisdn`).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_key", "channel_key", "msisdn", "product_id", "mcc", "mnc"}).
			AddRow("talentrivia", "timwe-gh-airteltigo-ghana", "233260000000", 33261, nil, nil))
	mock.ExpectExec(`UPDATE tenant_event_outbox\s+SET optout_status = \$3`).
		WithArgs(optoutTestTenant, optoutTestEvent, "done", sqlmock.AnyArg(), "HTTP 200").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.dispatchOneTenantOptout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("subscription-external opt-out called %d times; want 1", n)
	}
}
