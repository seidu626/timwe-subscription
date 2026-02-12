package worker

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	notifProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_processed_total",
			Help: "Total notifications processed by type",
		},
		[]string{"type"},
	)

	notifErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_errors_total",
			Help: "Total errors by notification type and stage",
		},
		[]string{"type", "stage"},
	)

	optinSuccessTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "optin_success_total",
			Help: "Total successful opt-ins executed by monitor",
		},
	)

	resubscribeSuccessTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "resubscribe_success_total",
			Help: "Total successful resubscribe operations",
		},
	)
)

func RegisterMetrics() {
	prometheus.MustRegister(notifProcessedTotal)
	prometheus.MustRegister(notifErrorsTotal)
	prometheus.MustRegister(optinSuccessTotal)
	prometheus.MustRegister(resubscribeSuccessTotal)
}

func incProcessed(t string) { notifProcessedTotal.WithLabelValues(t).Inc() }
func incError(t, stage string) { notifErrorsTotal.WithLabelValues(t, stage).Inc() }
func incOptinSuccess() { optinSuccessTotal.Inc() }
func incResubscribeSuccess() { resubscribeSuccessTotal.Inc() }
