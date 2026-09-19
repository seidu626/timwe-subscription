package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSubscriptionOnlyCapabilitiesRequireAuthentication(t *testing.T) {
	h := &SubscriptionHandler{batchGuard: &batchAdminGuard{internalSecret: testHMACSecret}}
	for _, authenticated := range []bool{false, true} {
		ctx := &fasthttp.RequestCtx{}
		if authenticated {
			ctx = makeHMACCtx(testHMACSecret, nil)
		}
		ctx.Request.Header.SetMethod("GET")
		ctx.Request.SetRequestURI("/batch?capabilities=1&tenant_key=nrg")
		h.BatchStatusHandler(ctx)
		if !authenticated {
			if ctx.Response.StatusCode() == 200 {
				t.Fatal("unauthenticated capabilities accepted")
			}
			continue
		}
		var caps map[string]bool
		if err := json.Unmarshal(ctx.Response.Body(), &caps); err != nil {
			t.Fatal(err)
		}
		if !caps["subscription_only"] || !caps["invalid_msisdn_logging"] || !caps["failure_receipts"] {
			t.Fatalf("missing capabilities: %v", caps)
		}
	}
}

func TestBatchOptinConcurrencyIsBoundedAcrossJobs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Application.Batch.MaxWorkersPerJob = 4
	cfg.Application.Batch.MaxConcurrentOptins = 2
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		config: cfg,
		jobs:   NewBatchJobManager(),
		processOptinFn: func(*domain.OptinRequest) error {
			started <- struct{}{}
			<-release
			return nil
		},
	}

	req := func(prefix string) *domain.BatchOptinRequest {
		return &domain.BatchOptinRequest{
			SubscriptionOnly: true,
			EntryChannel:     "WAP",
			TenantKey:        "nrg",
			ChannelKey:       "ch1",
			MSISDNS:          []string{prefix + "1", prefix + "2", prefix + "3", prefix + "4"},
		}
	}

	var wg sync.WaitGroup
	for _, jobID := range []string{"job-a", "job-b"} {
		status, jobCtx := h.jobs.CreateJobWithTenant(jobID, 4, "nrg", "ch1")
		wg.Add(1)
		go func(ctx context.Context, id string, st *BatchJobStatus, ctxReq *domain.BatchOptinRequest) {
			defer wg.Done()
			h.runBatchJob(ctx, id, st, ctxReq)
		}(jobCtx, jobID, status, req(jobID))
	}

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("configured concurrent opt-ins did not start")
		}
	}
	select {
	case <-started:
		t.Fatal("more than MaxConcurrentOptins ran across jobs")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	wg.Wait()

	for _, jobID := range []string{"job-a", "job-b"} {
		status, ok := h.jobs.GetJob(jobID)
		if !ok || status.State != BatchJobCompleted || status.Processed != 4 || status.Successful != 4 {
			t.Fatalf("job %s did not complete: %+v", jobID, status)
		}
	}
}

func TestBatchWorkerLimitUsesConfiguredCapAndSafeFallback(t *testing.T) {
	configured := &SubscriptionHandler{config: &config.Config{}}
	configured.config.Application.Batch.MaxWorkersPerJob = 3
	if got := configured.batchWorkerLimit(5000); got != 3 {
		t.Fatalf("configured worker limit = %d, want 3", got)
	}

	fallback := &SubscriptionHandler{}
	if got := fallback.batchWorkerLimit(5000); got != defaultBatchOptinConcurrency {
		t.Fatalf("fallback worker limit = %d, want %d", got, defaultBatchOptinConcurrency)
	}
}

func TestBatchFailureReceiptsRetainEveryFailureWithoutRawIdentity(t *testing.T) {
	acceptedAt := time.Date(2026, time.September, 19, 12, 34, 56, 789000000, time.UTC)
	var logs bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	logger := zap.New(zapcore.NewCore(encoder, zapcore.Lock(zapcore.AddSync(&logs)), zap.ErrorLevel))
	h := &SubscriptionHandler{
		logger: logger,
		jobs:   NewBatchJobManager(),
		processOptinFn: func(*domain.OptinRequest) error {
			return &service.AcceptedSubscriptionPersistenceError{
				ProviderRequestID:     "provider-request",
				ProviderTransactionID: "provider-transaction",
				ExternalTransactionID: "external-transaction",
				TrackingID:            "request-tracking-uuid",
				ProviderAcceptedAt:    acceptedAt,
				FailureStage:          "subscription_upsert",
				TenantID:              "tenant-id",
				ChannelID:             "channel-id",
				ProductID:             32535,
				SubscriptionResult:    "OPTIN_ALREADY_ACTIVE",
				Attempts:              3,
				PostgresCode:          "53300",
				Transient:             true,
			}
		},
	}
	rawMSISDNs := []string{"233241234567", "233241234568", "233241234569"}
	status, jobCtx := h.jobs.CreateJobWithTenant("receipt-job", len(rawMSISDNs), "nrg", "ch1")
	h.runBatchJob(jobCtx, "receipt-job", status, &domain.BatchOptinRequest{
		SubscriptionOnly: true,
		EntryChannel:     "WAP",
		TenantKey:        "nrg",
		ChannelKey:       "ch1",
		MSISDNS:          rawMSISDNs,
	})

	snapshot, ok := h.jobs.GetJob("receipt-job")
	if !ok || snapshot.State != BatchJobFailed || snapshot.Failed != int64(len(rawMSISDNs)) || len(snapshot.Failures) != len(rawMSISDNs) {
		t.Fatalf("incomplete failure receipt: %+v", snapshot)
	}
	indices := make(map[int]bool, len(rawMSISDNs))
	for _, failure := range snapshot.Failures {
		indices[failure.ItemIndex] = true
		if failure.IdentityHash == "" || !failure.AcceptedByProvider || failure.ProviderRequestID != "provider-request" ||
			failure.ProviderTransactionID != "provider-transaction" || failure.ExternalTransactionID != "external-transaction" ||
			failure.TrackingID != "request-tracking-uuid" || failure.ProviderAcceptedAt != acceptedAt.Format(time.RFC3339Nano) ||
			failure.LocalPersistenceStage != "subscription_upsert" ||
			failure.PostgresCode != "53300" || failure.PersistenceAttempts != 3 {
			t.Fatalf("incomplete safe failure: %+v", failure)
		}
	}
	for index := range rawMSISDNs {
		if !indices[index] {
			t.Fatalf("missing zero-based submitted item index %d", index)
		}
	}
	if snapshot.ErrorDetails["trackingId"] != "request-tracking-uuid" || snapshot.ErrorDetails["providerAcceptedAt"] != acceptedAt.Format(time.RFC3339Nano) {
		t.Fatalf("first error details omitted reconciliation evidence: %+v", snapshot.ErrorDetails)
	}

	receiptJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, rawMSISDN := range rawMSISDNs {
		if strings.Contains(string(receiptJSON), rawMSISDN) || strings.Contains(logs.String(), rawMSISDN) {
			t.Fatalf("raw MSISDN %s leaked into receipt or logs", rawMSISDN)
		}
	}
	if strings.Count(logs.String(), "Batch subscription item failed") != len(rawMSISDNs) {
		t.Fatalf("worker failures were sampled or dropped: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "request-tracking-uuid") || !strings.Contains(logs.String(), acceptedAt.Format(time.RFC3339Nano)) {
		t.Fatalf("structured failure logs omitted reconciliation evidence: %s", logs.String())
	}
}

func TestSafeBatchItemFailureRetainsUnknownProviderOutcomeCorrelation(t *testing.T) {
	failure := safeBatchItemFailure("job", 4, "233241234567", &service.SubscriptionOnlyProviderOutcomeUnknownError{
		ExternalTransactionID: "provider-attempt-header-id",
		TrackingID:            "local-request-tracking-id",
		ProviderRequestID:     "provider-request-id",
		TenantID:              "tenant-id",
		ChannelID:             "channel-id",
		ProductID:             32535,
	})

	if failure.Code != "provider_outcome_unknown" || failure.Outcome != "outcome_unknown" || failure.AcceptedByProvider ||
		failure.ExternalTransactionID != "provider-attempt-header-id" || failure.TrackingID != "local-request-tracking-id" ||
		failure.ProviderRequestID != "provider-request-id" || failure.TenantID != "tenant-id" ||
		failure.ChannelID != "channel-id" || failure.ProductID != 32535 {
		t.Fatalf("unknown provider outcome receipt lost safe correlation: %+v", failure)
	}
	encoded, err := json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "233241234567") {
		t.Fatalf("unknown provider outcome receipt leaked raw identity: %s", encoded)
	}
}

func TestSafeBatchItemFailureClassifiesAcceptedMaterializationFailure(t *testing.T) {
	acceptedAt := time.Date(2026, time.September, 19, 15, 0, 0, 0, time.UTC)
	failure := safeBatchItemFailure("job", 1, "233241234567", &service.AcceptedSubscriptionPersistenceError{
		ProviderRequestID:     "provider-request-id",
		ExternalTransactionID: "provider-attempt-header-id",
		TrackingID:            "local-request-tracking-id",
		ProviderAcceptedAt:    acceptedAt,
		FailureStage:          "provider_transaction_id",
		TenantID:              "tenant-id",
		ChannelID:             "channel-id",
		ProductID:             32535,
		SubscriptionResult:    "OPTIN_ALREADY_ACTIVE",
	})
	if failure.Code != "accepted_materialization_failed" || !failure.AcceptedByProvider ||
		failure.ProviderTransactionID != "" || failure.LocalPersistenceStage != "provider_transaction_id" ||
		failure.ExternalTransactionID != "provider-attempt-header-id" || failure.TrackingID != "local-request-tracking-id" ||
		failure.ProviderAcceptedAt != acceptedAt.Format(time.RFC3339Nano) {
		t.Fatalf("accepted materialization receipt incomplete: %+v", failure)
	}
}

func TestSafeBatchItemFailureClassifiesLocalRegistryFailures(t *testing.T) {
	for _, tt := range []struct {
		name         string
		outcome      string
		stage        string
		providerCode string
		wantCode     string
	}{
		{name: "provider rejected blacklist", outcome: "provider_rejected", stage: "blacklist_registry", providerCode: "BLACKLISTED", wantCode: "local_evidence_persistence_failed"},
		{name: "provider not attempted", outcome: "provider_not_attempted", stage: "tenant_exclusion_read", wantCode: "local_preflight_read_failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			failure := safeBatchItemFailure("job", 2, "233241234567", &service.SubscriptionOnlyLocalEvidenceError{
				Outcome:               tt.outcome,
				ProviderCode:          tt.providerCode,
				TenantID:              "tenant-id",
				ChannelID:             "channel-id",
				LocalPersistenceStage: tt.stage,
				Attempts:              1,
				PostgresCode:          "53300",
				Transient:             true,
			})
			if failure.Code != tt.wantCode || failure.Outcome != tt.outcome || failure.ProviderCode != tt.providerCode ||
				failure.LocalPersistenceStage != tt.stage || failure.PostgresCode != "53300" || !failure.TransientPersistence ||
				failure.AcceptedByProvider {
				t.Fatalf("local registry receipt incomplete: %+v", failure)
			}
			encoded, err := json.Marshal(failure)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), `"acceptedByProvider":false`) {
				t.Fatalf("receipt omitted explicit provider acceptance classification: %s", encoded)
			}
		})
	}
}

func TestSubscriptionOnlyBatchPropagationAndLiveProgress(t *testing.T) {
	requests := make(chan *domain.OptinRequest, 2)
	release := make(chan struct{})
	defer close(release)
	h := &SubscriptionHandler{logger: zap.NewNop(), jobs: NewBatchJobManager(), batchGuard: &batchAdminGuard{internalSecret: testHMACSecret}, processOptinFn: func(req *domain.OptinRequest) error {
		requests <- req
		<-release
		return nil
	}}
	for _, mode := range []string{"", "SMS", "sms"} {
		body, _ := json.Marshal(map[string]interface{}{"subscription_only": true, "msisdns": []string{"233241234567"}, "entry_channel": mode, "tenant_key": "nrg"})
		ctx := makeHMACCtx(testHMACSecret, body)
		h.BatchOptinHandler(ctx)
		if ctx.Response.StatusCode() != 422 {
			t.Fatalf("mode %q status %d", mode, ctx.Response.StatusCode())
		}
	}
	body, _ := json.Marshal(map[string]interface{}{"subscription_only": true, "msisdns": []string{"233241234567"}, "entry_channel": "WAP", "tenant_key": "nrg", "channel_key": "ch1", "telco": "MTN"})
	ctx := makeHMACCtx(testHMACSecret, body)
	h.BatchOptinHandler(ctx)
	if ctx.Response.StatusCode() != 202 {
		t.Fatalf("status %d: %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var accepted map[string]string
	json.Unmarshal(ctx.Response.Body(), &accepted)
	select {
	case req := <-requests:
		if !req.SubscriptionOnly || req.TenantRoute.TenantKey != "nrg" || req.TenantRoute.ChannelKey != "ch1" {
			t.Fatalf("lost controls: %+v", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no optin")
	}
	snapshot, ok := h.jobs.GetJob(accepted["jobId"])
	if !ok || snapshot.State != BatchJobRunning || snapshot.Total != 1 || snapshot.Processed != 0 {
		t.Fatalf("bad running snapshot: %+v", snapshot)
	}
	release <- struct{}{}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := h.jobs.GetJob(accepted["jobId"])
		if st.State == BatchJobCompleted {
			if st.Processed != 1 || st.Successful != 1 || st.Failed != 0 {
				t.Fatalf("bad final counters: %+v", st)
			}
			if snapshot.Processed != 0 {
				t.Fatal("prior snapshot mutated")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
}
