package repository

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func expectEventClaim(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`WITH due AS .* UPDATE tenant_event_outbox o\s+SET status = 'sending'`).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "tenant_id", "payload", "attempts"}).
			AddRow(optoutTestEvent, optoutTestTenant, []byte(`{"type":"CHARGE_SUCCESS","msisdn":"233260000000"}`), 1))
}

func expectBatchSubscriber(mock sqlmock.Sqlmock, batch bool) {
	mock.ExpectQuery(`SELECT COALESCE\(\(SELECT s.campaign_url = 'INTERNAL'`).
		WithArgs(optoutTestTenant, optoutTestEvent).
		WillReturnRows(sqlmock.NewRows([]string{"batch"}).AddRow(batch))
}

// stubWebhook counts deliveries to the tenant's webhook.
func stubWebhook(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

func TestDispatchTenantEvent_BatchSubscriberSkippedUnlessEnabled(t *testing.T) {
	cases := []struct {
		name           string
		enabled, batch bool
		wantSent       bool
	}{
		{"batch subscriber, default off", false, true, false},
		{"batch subscriber, explicitly on", true, true, true},
		{"tenant subscriber, default off", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, hits := stubWebhook(t)
			bindTestTenantEventsWith(t, url, map[string]any{"send_batch_subscriber_events": tc.enabled})
			repo, mock := newNotificationRepoWithMock(t)
			expectEventClaim(mock)
			expectActiveBinding(mock, true)
			if !tc.enabled {
				expectBatchSubscriber(mock, tc.batch)
			}
			if tc.wantSent {
				mock.ExpectExec(`UPDATE tenant_event_outbox\s+SET status = \$3`).
					WithArgs(optoutTestTenant, optoutTestEvent, "delivered", sqlmock.AnyArg(), 200, "", `{"ok":true}`, false).
					WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec(`UPDATE tenant_event_outbox SET status = 'skipped'`).
					WithArgs(optoutTestTenant, optoutTestEvent, "BATCH_SUBSCRIBER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			}

			claimed, err := repo.dispatchOneTenantEvent(context.Background())
			if err != nil || !claimed {
				t.Fatalf("dispatchOneTenantEvent = %v, %v; want claimed, nil", claimed, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			if sent := hits.Load() > 0; sent != tc.wantSent {
				t.Fatalf("webhook called = %v; want %v", sent, tc.wantSent)
			}
		})
	}
}

func TestDispatchTenantEvent_BatchLookupErrorDoesNotSend(t *testing.T) {
	url, hits := stubWebhook(t)
	bindTestTenantEventsWith(t, url, nil)
	repo, mock := newNotificationRepoWithMock(t)
	expectEventClaim(mock)
	expectActiveBinding(mock, true)
	mock.ExpectQuery(`SELECT COALESCE\(\(SELECT s.campaign_url = 'INTERNAL'`).
		WithArgs(optoutTestTenant, optoutTestEvent).
		WillReturnError(errors.New("pq: sorry, too many clients already"))

	claimed, err := repo.dispatchOneTenantEvent(context.Background())
	if err == nil || !claimed {
		t.Fatalf("dispatchOneTenantEvent = %v, %v; want claimed with an error", claimed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("webhook called %d times; want 0", n)
	}
}
