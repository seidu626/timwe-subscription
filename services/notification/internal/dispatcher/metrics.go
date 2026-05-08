package dispatcher

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/observability"
)

var (
	dispatchMetricsOnce sync.Once
	dispatchTotal       = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_worker_dispatch_total",
			Help: "Total notification worker dispatch outcomes by safe tenant and channel labels.",
		},
		[]string{"tenant_id", "channel_id", "worker", "status"},
	)
)

func RegisterMetrics() {
	dispatchMetricsOnce.Do(func() {
		prometheus.MustRegister(dispatchTotal)
	})
}

func recordDispatch(job domain.OutboxJob, status string) {
	labels := observability.WorkerLabels(job.TenantID, job.ChannelID, "notification_worker")
	dispatchTotal.WithLabelValues(labels.TenantID, labels.ChannelID, labels.Worker, observability.SafeLabelValue(status)).Inc()
}
