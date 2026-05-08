package dispatcher

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"go.uber.org/zap"
)

func TestRecordDispatchUsesSafeTenantChannelLabels(t *testing.T) {
	tenantID := "tenant-a"
	channelID := "sms"
	job := domain.OutboxJob{
		JobID:     "job-1",
		TenantID:  &tenantID,
		ChannelID: &channelID,
	}

	recordDispatch(job, "sent")

	metric, err := dispatchTotal.GetMetricWithLabelValues("tenant-a", "sms", "notification_worker", "sent")
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues: %v", err)
	}
	var got dto.Metric
	if err := metric.Write(&got); err != nil {
		t.Fatalf("metric Write: %v", err)
	}
	if got.Counter == nil || got.Counter.GetValue() < 1 {
		t.Fatalf("expected dispatch counter to increment, got %#v", got.Counter)
	}
}

func TestJobFieldsExcludePII(t *testing.T) {
	tenantID := "tenant-a"
	channelID := "sms"
	dispatcher := NewDispatcher(nil, zap.NewNop(), Config{})
	fields := dispatcher.jobFields(domain.OutboxJob{
		JobID:     "job-1",
		TenantID:  &tenantID,
		ChannelID: &channelID,
		MSISDN:    "233241234567",
	})

	for _, field := range fields {
		if field.Key == "msisdn" || field.Key == "click_id" || field.Key == "secret" {
			t.Fatalf("error: PII field must not be logged: %s", field.Key)
		}
	}
}
