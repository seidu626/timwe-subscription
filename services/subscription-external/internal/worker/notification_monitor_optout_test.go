package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

// optoutRepo serves one USER_OPTOUT row for an active subscription. The
// embedded interface is nil, so any repository call the opt-out path is not
// expected to make panics the test.
type optoutRepo struct {
	repository.SubscriptionRepositoryInterface
	rows     []repository.NotificationRow
	statuses []string
}

func (r *optoutRepo) FetchUnprocessedOptoutNotifications(time.Time, int64, int) ([]repository.NotificationRow, error) {
	rows := r.rows
	r.rows = nil
	return rows, nil
}

func (r *optoutRepo) GetSubscriptionByMSISDNAndProduct(string, int) (*domain.Subscription, error) {
	return &domain.Subscription{Status: "active"}, nil
}

func (r *optoutRepo) GetLastOptinNotificationTime(string, int) (*time.Time, error) {
	return nil, nil
}

func (r *optoutRepo) UpsertSubscriptionStatus(_ string, _ int, status string) error {
	r.statuses = append(r.statuses, status)
	return nil
}

type optoutRedis struct {
	cached.RedisClient
}

func (optoutRedis) Get(context.Context, string) (string, error)      { return "", errors.New("miss") }
func (optoutRedis) Exists(context.Context, ...string) (int64, error) { return 0, nil }
func (optoutRedis) SetNX(context.Context, string, interface{}, time.Duration) (bool, error) {
	return true, nil
}
func (optoutRedis) Set(context.Context, string, interface{}, time.Duration) error { return nil }
func (optoutRedis) Del(context.Context, ...string) (int64, error)                 { return 1, nil }

func TestProcessUserOptoutMarksInactiveWithoutReoptin(t *testing.T) {
	repo := &optoutRepo{rows: []repository.NotificationRow{{
		ID: 1, MSISDN: "233241234567", ProductID: 32535, EntryChannel: "WAP",
		CreatedAt: time.Now().Add(-time.Hour), Type: "USER_OPTOUT",
	}}}
	// userSvc is nil and EntryChannels is set: a re-opt-in attempt would
	// reach the nil subscription service and fail the test.
	m := &NotificationMonitor{
		logger: zap.NewNop(),
		repo:   repo,
		redis:  optoutRedis{},
		ctx:    context.Background(),
		cfg: NotificationMonitorConfig{
			ProductIds:       []string{"32535"},
			EntryChannels:    []string{"WAP"},
			MaxRetries:       1,
			ScanLookbackDays: 15,
			RedisKeyPrefix:   "test",
		},
		circuitBreakerState: "CLOSED",
	}

	if err := m.processUserOptout(); err != nil {
		t.Fatalf("processUserOptout: %v", err)
	}
	if len(repo.statuses) != 1 || repo.statuses[0] != "inactive" {
		t.Fatalf("expected a single inactive upsert and no re-activation, got %v", repo.statuses)
	}
}
