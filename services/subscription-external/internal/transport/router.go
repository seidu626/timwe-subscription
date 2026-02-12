package transport

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/subscription-external/internal/handler"
	fastHttpSwagger "github.com/swaggo/fasthttp-swagger"
	"github.com/valyala/fasthttp"
)

func NewRouter(subscriptionHandler *handler.SubscriptionHandler, userBaseHandler *handler.UserBaseHandler, partnerHandler *handler.PartnerHandler, monitoringHandler *handler.MonitoringHandler, workerHandler *handler.WorkerHandler, renewalHandler *handler.RenewalHandler, notificationWebhookHandler *handler.NotificationWebhookHandler) fasthttp.RequestHandler {
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		method := string(ctx.Method())

		switch {
		case strings.HasPrefix(path, "/swagger"):
			fastHttpSwagger.WrapHandler(fastHttpSwagger.InstanceName("swagger"))(ctx)
		case strings.EqualFold(path, "/health"):
			if method == fasthttp.MethodGet {
				handler.HealthCheck(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/health/msisdn-generator"):
			if method == fasthttp.MethodGet {
				subscriptionHandler.HealthCheckHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/metrics"):
			if method == fasthttp.MethodGet {
				handler.MetricsHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/userbase/upload"):
			if method == fasthttp.MethodPost {
				userBaseHandler.UploadHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/subscription-external/batch"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.BatchOptinHandler(ctx)
			} else if method == fasthttp.MethodGet {
				subscriptionHandler.BatchStatusHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/subscription-external"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.OptinHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/subscription-external/backfill"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.BackfillOptinHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/subscription-external/resubscribe"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.ResubscribeHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}

			// Enhanced resubscribe endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/resubscribe/enhanced"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.EnhancedResubscribeHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Charging failures analysis endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/charging-failures"):
			if method == fasthttp.MethodGet {
				subscriptionHandler.GetChargingFailuresHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Charging failure statistics endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/charging-failures/stats"):
			if method == fasthttp.MethodGet {
				subscriptionHandler.GetChargingFailureStatsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Charging failure summary endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/charging-failures/summary"):
			if method == fasthttp.MethodGet {
				subscriptionHandler.GetChargingFailureSummaryHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Renewal system endpoints
		case strings.EqualFold(path, "/api/v1/renewal/worker/start"):
			if method == fasthttp.MethodPost {
				renewalHandler.StartRenewalWorker(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/worker/stop"):
			if method == fasthttp.MethodPost {
				renewalHandler.StopRenewalWorker(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/worker/status"):
			if method == fasthttp.MethodGet {
				renewalHandler.GetRenewalWorkerStatus(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/statistics"):
			if method == fasthttp.MethodGet {
				renewalHandler.GetRenewalStatistics(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/churn-candidates"):
			if method == fasthttp.MethodGet {
				renewalHandler.GetChurnCandidates(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/priority-retry/process"):
			if method == fasthttp.MethodPost {
				renewalHandler.ProcessPriorityRetryQueue(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/manual"):
			if method == fasthttp.MethodPost {
				renewalHandler.ManualRenewal(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/health"):
			if method == fasthttp.MethodGet {
				renewalHandler.GetRenewalHealth(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/renewal/force-churn-evaluation"):
			if method == fasthttp.MethodPost {
				renewalHandler.ForceChurnEvaluation(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Charging failure by MSISDN endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/charging-failures/msisdn"):
			if method == fasthttp.MethodGet {
				subscriptionHandler.GetChargingFailureByMSISDNHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Update charging health status endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/charging-failures/health-status"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.UpdateChargingHealthStatusHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Mark charging failure as processed endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/charging-failures/mark-processed"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.MarkChargingFailureAsProcessedHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Monitoring endpoints
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/dashboard"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetDashboardDataHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// WebSocket endpoint for real-time monitoring
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/ws"):
			if method == fasthttp.MethodGet {
				monitoringHandler.HandleWebSocketConnection(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Health check endpoint
		case strings.EqualFold(path, "/health"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetHealthHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Historical data endpoints
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/historical"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetHistoricalMetricsHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/trends"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetTrendAnalysisHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/summary"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetMetricsSummaryHandler(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// HTML Dashboard endpoint
		case strings.EqualFold(path, "/dashboard"):
			if method == fasthttp.MethodGet {
				// Serve the actual dashboard.html file
				ctx.SetContentType("text/html")
				ctx.SetStatusCode(fasthttp.StatusOK)

				// Read the dashboard.html file from the filesystem
				dashboardPath := "internal/monitoring/dashboard.html"
				dashboardContent, err := os.ReadFile(dashboardPath)
				if err != nil {
					// Fallback to simple dashboard if file can't be read
					ctx.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>TIMWE Dashboard</title>
</head>
<body>
    <h1>TIMWE Monitoring Dashboard</h1>
    <p>Error: Could not load dashboard.html file</p>
    <p><a href="/api/v1/subscription-external/monitoring/dashboard">View Dashboard Data (JSON)</a></p>
    <p><a href="/api/v1/subscription-external/monitoring/metrics">View Metrics (JSON)</a></p>
    <p><a href="/api/v1/subscription-external/monitoring/health">View Health Status (JSON)</a></p>
    <p><a href="/swagger/index.html">API Documentation</a></p>
</body>
</html>`)
				} else {
					ctx.Write(dashboardContent)
				}
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Monitoring metrics endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/metrics"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetMetricsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Monitoring alerts endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/alerts"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetAlertsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Acknowledge alert endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/alerts/acknowledge"):
			if method == fasthttp.MethodPost {
				monitoringHandler.AcknowledgeAlertHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Clear alerts endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/alerts/clear"):
			if method == fasthttp.MethodPost {
				monitoringHandler.ClearAlertsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Update thresholds endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/thresholds"):
			if method == fasthttp.MethodPost {
				monitoringHandler.UpdateThresholdsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Health check endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/monitoring/health"):
			if method == fasthttp.MethodGet {
				monitoringHandler.GetHealthHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker start endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/start"):
			if method == fasthttp.MethodPost {
				workerHandler.StartProcessingHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker stop endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/stop"):
			if method == fasthttp.MethodPost {
				workerHandler.StopProcessingHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker status endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/status"):
			if method == fasthttp.MethodGet {
				workerHandler.GetProcessingStatusHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker stats endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/stats"):
			if method == fasthttp.MethodGet {
				workerHandler.GetProcessingStatsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker results endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/results"):
			if method == fasthttp.MethodGet {
				workerHandler.GetProcessingResultsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker progress endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/progress"):
			if method == fasthttp.MethodGet {
				workerHandler.GetProcessingProgressHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker config endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/config"):
			if method == fasthttp.MethodGet {
				workerHandler.GetProcessingConfigHandler(ctx)
			} else if method == fasthttp.MethodPost {
				workerHandler.UpdateProcessingConfigHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker pause endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/pause"):
			if method == fasthttp.MethodPost {
				workerHandler.PauseProcessingHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker resume endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/resume"):
			if method == fasthttp.MethodPost {
				workerHandler.ResumeProcessingHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker detailed status endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/detailed-status"):
			if method == fasthttp.MethodGet {
				workerHandler.GetDetailedStatusHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker summary endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/summary"):
			if method == fasthttp.MethodGet {
				workerHandler.GetProcessingSummaryHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker export endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/export"):
			if method == fasthttp.MethodGet {
				workerHandler.ExportResultsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker time range results endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/results/time-range"):
			if method == fasthttp.MethodGet {
				workerHandler.GetResultsByTimeRangeHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker clear results endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/results/clear"):
			if method == fasthttp.MethodPost {
				workerHandler.ClearResultsHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Worker graceful shutdown endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/worker/shutdown"):
			if method == fasthttp.MethodPost {
				workerHandler.GracefulShutdownHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Batch progress endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/batch/progress"):
			if method == fasthttp.MethodGet {
				subscriptionHandler.GetBatchProgressHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Emergency stop endpoint
		case strings.EqualFold(path, "/api/v1/subscription-external/batch/stop"):
			if method == fasthttp.MethodPost {
				subscriptionHandler.StopBatchHandler(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
			return
		// Partner MT: /api/external/v1/{channel}/mt
		case strings.HasPrefix(path, "/api/external/v1/") && strings.HasSuffix(path, "/mt"):
			if method != fasthttp.MethodPost {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
				return
			}
			// Expected structure: /api/external/v1/{channel}/mt
			segments := strings.Split(strings.TrimPrefix(path, "/api/external/v1/"), "/")
			if len(segments) != 2 || segments[1] != "mt" {
				ctx.Error("Bad Request", fasthttp.StatusBadRequest)
				return
			}
			channel := segments[0]
			partnerHandler.PartnerMTHandler(ctx, channel)
		// Partner Charge: /api/external/v1/charge/dob
		case strings.EqualFold(path, "/api/external/v1/charge/dob"):
			if method != fasthttp.MethodPost {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
				return
			}
			partnerHandler.PartnerChargeHandler(ctx)
		// Partner Status: /api/external/v1/subscription/status
		case strings.EqualFold(path, "/api/external/v1/subscription/status"):
			if method != fasthttp.MethodPost {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
				return
			}
			partnerHandler.PartnerStatusHandler(ctx)
		// Partner Optout: /api/external/v1/subscription/optout
		case strings.EqualFold(path, "/api/external/v1/subscription/optout"):
			if method != fasthttp.MethodPost {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
				return
			}
			partnerHandler.PartnerOptoutHandler(ctx)
		// Partner Optin Confirm: /api/external/v1/subscription/optin/confirm
		case strings.EqualFold(path, "/api/external/v1/subscription/optin/confirm"):
			if method != fasthttp.MethodPost {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
				return
			}
			partnerHandler.PartnerOptinConfirmHandler(ctx)

		// TIMWE Notification Webhook: /api/v1/webhooks/timwe/notification
		// Receives CHARGE, USER_RENEWED, USER_OPTIN, USER_OPTOUT notifications from TIMWE
		case strings.EqualFold(path, "/api/v1/webhooks/timwe/notification"):
			if method != fasthttp.MethodPost {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
				return
			}
			if notificationWebhookHandler != nil {
				notificationWebhookHandler.HandleNotificationWebhook(ctx)
			} else {
				ctx.Error("Webhook handler not configured", fasthttp.StatusServiceUnavailable)
			}
			return

		default:
			log.Printf("Processing unknown request: %s", ctx.Request.String())
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}
	return router
}

// Helper function to extract the partnerRoleId from the path
func extractPartnerRoleId(path, prefix string) string {
	if len(path) > len(prefix) {
		partnerRoleId := path[len(prefix):]
		// Check if the extracted part is a valid number
		if _, err := strconv.Atoi(partnerRoleId); err == nil {
			return partnerRoleId
		}
	}
	return ""
}
