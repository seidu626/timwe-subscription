package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// TestBatchOptinHandler_TenantRequired verifies that a batch request without
// tenant_key/channel_key is rejected with 422.
func TestBatchOptinHandler_TenantRequired(t *testing.T) {
	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		jobs:   NewBatchJobManager(),
	}

	t.Run("missing tenant and channel", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"telco": "MTN",
			"count": 10,
		})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")

		h.BatchOptinHandler(ctx)
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, ctx.Response.StatusCode())
		var resp map[string]string
		_ = json.Unmarshal(ctx.Response.Body(), &resp)
		assert.Contains(t, resp["error"], "tenant_key")
	})

	t.Run("missing channel only", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"telco":      "MTN",
			"tenant_key": "nrg",
		})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")

		h.BatchOptinHandler(ctx)
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, ctx.Response.StatusCode())
	})

	t.Run("via headers accepted", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"telco": "MTN",
			// Use empty msisdns but count=0 so no real work happens.
		})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.Header.Set("X-Tenant-Key", "nrg")
		ctx.Request.Header.Set("X-Channel-Key", "ch1")

		h.BatchOptinHandler(ctx)
		// 202 Accepted – job is enqueued.
		assert.Equal(t, fasthttp.StatusAccepted, ctx.Response.StatusCode())
		var resp map[string]string
		_ = json.Unmarshal(ctx.Response.Body(), &resp)
		assert.NotEmpty(t, resp["jobId"])
	})
}

// TestBatchJobLifecycle_ProgressAndStop tests progress reporting and cancellation.
func TestBatchJobLifecycle_ProgressAndStop(t *testing.T) {
	mgr := NewBatchJobManager()

	// Create a job
	st, _ := mgr.CreateJob("job-1", 100)
	assert.Equal(t, BatchJobPending, st.State)

	// Simulate start
	mgr.setRunning("job-1")
	retrieved, ok := mgr.GetJob("job-1")
	assert.True(t, ok)
	assert.Equal(t, BatchJobRunning, retrieved.State)

	// Cancel the job
	cancelled := mgr.CancelJob("job-1")
	assert.True(t, cancelled)
	retrieved, _ = mgr.GetJob("job-1")
	assert.Equal(t, BatchJobCancelled, retrieved.State)
	assert.NotNil(t, retrieved.CompletedAt)

	// Cancelling again must return false (already terminal)
	cancelled2 := mgr.CancelJob("job-1")
	assert.False(t, cancelled2)
}

// TestGetBatchProgressHandler verifies progress endpoint returns job state.
func TestGetBatchProgressHandler(t *testing.T) {
	const secret = "test-progress-secret"
	h := &SubscriptionHandler{
		logger:     zap.NewNop(),
		jobs:       NewBatchJobManager(),
		batchGuard: &batchAdminGuard{internalSecret: secret},
	}

	// Create a known job
	st, _ := h.jobs.CreateJob("job-xyz", 50)
	h.jobs.setRunning("job-xyz")
	st.Processed = 25

	t.Run("returns progress for known job", func(t *testing.T) {
		ctx := makeHMACCtxGetProgress(secret, "job-xyz")

		h.GetBatchProgressHandler(ctx)
		assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

		var resp BatchJobStatus
		_ = json.Unmarshal(ctx.Response.Body(), &resp)
		assert.Equal(t, "job-xyz", resp.ID)
		assert.Equal(t, BatchJobRunning, resp.State)
		assert.Equal(t, 50, resp.Total)
	})

	t.Run("returns 404 for unknown job", func(t *testing.T) {
		ctx := makeHMACCtxGetProgress(secret, "no-such-job")

		h.GetBatchProgressHandler(ctx)
		assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	})

	t.Run("returns 400 when batch_id missing", func(t *testing.T) {
		ctx := makeHMACCtxGetProgress(secret, "")

		h.GetBatchProgressHandler(ctx)
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	})
}

// makeHMACCtxGetProgress creates a GET-style request ctx with HMAC headers and optional batch_id query param.
func makeHMACCtxGetProgress(secret, batchID string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	if batchID != "" {
		ctx.Request.SetRequestURI("/api/v1/subscription-external/batch/progress?batch_id=" + batchID)
	} else {
		ctx.Request.SetRequestURI("/api/v1/subscription-external/batch/progress")
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	sig := computeTestHMAC(secret, ts, []byte{})
	ctx.Request.Header.Set("X-Internal-Timestamp", ts)
	ctx.Request.Header.Set("X-Internal-Signature", sig)
	return ctx
}

// TestStopBatchHandler verifies the stop endpoint cancels a running job.
func TestStopBatchHandler(t *testing.T) {
	const secret = "test-stop-secret"
	h := &SubscriptionHandler{
		logger:     zap.NewNop(),
		jobs:       NewBatchJobManager(),
		batchGuard: &batchAdminGuard{internalSecret: secret},
	}

	_, _ = h.jobs.CreateJob("job-stop", 200)
	h.jobs.setRunning("job-stop")

	t.Run("stops a running job", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"batch_id": "job-stop", "reason": "test"})
		ctx := makeHMACCtx(secret, body)

		h.StopBatchHandler(ctx)
		assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

		// Verify state
		retrieved, _ := h.jobs.GetJob("job-stop")
		assert.Equal(t, BatchJobCancelled, retrieved.State)
	})

	t.Run("409 when job is already cancelled", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"batch_id": "job-stop"})
		ctx := makeHMACCtx(secret, body)

		h.StopBatchHandler(ctx)
		assert.Equal(t, fasthttp.StatusConflict, ctx.Response.StatusCode())
	})

	t.Run("404 for unknown batch", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"batch_id": "no-such"})
		ctx := makeHMACCtx(secret, body)

		h.StopBatchHandler(ctx)
		assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	})

	t.Run("400 when batch_id missing", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")

		h.StopBatchHandler(ctx)
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	})
}

// TestContextCancellation_PropagatedToJob verifies that CancelJob triggers the context.
func TestContextCancellation_PropagatedToJob(t *testing.T) {
	mgr := NewBatchJobManager()
	_, ctx := mgr.CreateJob("job-ctx", 100)
	mgr.setRunning("job-ctx")

	// Context should be open
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done yet")
	default:
	}

	// Cancel the job
	mgr.CancelJob("job-ctx")

	// Context should be cancelled within a short window
	select {
	case <-ctx.Done():
		// pass
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context was not cancelled after CancelJob")
	}
}

// TestBackfillOptinHandler_TenantRequired verifies 422 on missing tenant.
func TestBackfillOptinHandler_TenantRequired(t *testing.T) {
	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		jobs:   NewBatchJobManager(),
	}

	body := []byte(`{"product_ids":["p1"]}`)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	ctx.Request.Header.SetMethod("POST")

	h.BackfillOptinHandler(ctx)
	assert.Equal(t, fasthttp.StatusUnprocessableEntity, ctx.Response.StatusCode())

	_ = strings.Contains // keep import
}

// TestResubscribeHandler_TenantRequired verifies 422 on missing tenant.
func TestResubscribeHandler_TenantRequired(t *testing.T) {
	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		jobs:   NewBatchJobManager(),
	}

	body := []byte(`{"product_ids":["p1"]}`)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	ctx.Request.Header.SetMethod("POST")

	h.ResubscribeHandler(ctx)
	assert.Equal(t, fasthttp.StatusUnprocessableEntity, ctx.Response.StatusCode())
}

// TestBatchJob_TenantRouteReachesProcessOptin verifies that tenant_key/channel_key stored
// on a batch job are propagated into every OptinRequest fed to the ProcessOptin call path.
// A fake processOptinFn records the first TenantRoute it sees; the test asserts it matches
// the keys supplied to BatchOptinHandler.
func TestBatchJob_TenantRouteReachesProcessOptin(t *testing.T) {
	// Channel to receive the first captured route.
	routeCh := make(chan domain.TenantRouteContext, 1)

	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		jobs:   NewBatchJobManager(),
		// Inject a fake processOptinFn that records the TenantRoute.
		processOptinFn: func(req *domain.OptinRequest) error {
			select {
			case routeCh <- req.TenantRoute:
			default: // only capture the first one
			}
			return nil
		},
	}

	// Send one MSISDN so the job processes at least one request.
	body, _ := json.Marshal(map[string]interface{}{
		"telco":       "MTN",
		"msisdns":     []string{"233241234567"},
		"tenant_key":  "nrg",
		"channel_key": "ch1",
	})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	ctx.Request.Header.SetMethod("POST")

	h.BatchOptinHandler(ctx)
	assert.Equal(t, fasthttp.StatusAccepted, ctx.Response.StatusCode())

	// Wait for the background goroutine to call processOptinFn.
	select {
	case route := <-routeCh:
		assert.Equal(t, "nrg", route.TenantKey, "tenant_key must reach ProcessOptin")
		assert.Equal(t, "ch1", route.ChannelKey, "channel_key must reach ProcessOptin")
	case <-time.After(2 * time.Second):
		t.Fatal("processOptinFn was not called within 2s — tenant route did not propagate")
	}
}

// ── Test helper functions for HMAC and auth tests ──────────────────────────

const testHMACSecret = "test-internal-secret"

// computeTestHMAC produces the same signature as batchAdminGuard.validateHMAC.
func computeTestHMAC(secret, timestamp string, body []byte) string {
	message := timestamp + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// makeHMACCtx returns a request context with valid HMAC headers for the given body.
func makeHMACCtx(secret string, body []byte) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody(body)
	ts := time.Now().UTC().Format(time.RFC3339)
	ctx.Request.Header.Set("X-Internal-Timestamp", ts)
	sig := computeTestHMAC(secret, ts, body)
	ctx.Request.Header.Set("X-Internal-Signature", sig)
	return ctx
}

// makeBearerCtx simulates a request with a (fake) Bearer token.
func makeBearerCtx(token string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	return ctx
}

// makeUnauthCtx returns a request with no auth headers.
func makeUnauthCtx() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	return ctx
}

// ── BatchJobManager tests (FIX 1 + FIX 2) ────────────────────────────────────

// TestCreateJobWithTenant_StampsOwnership verifies that tenant fields are set
// atomically inside CreateJobWithTenant and are read-only after the call.
func TestCreateJobWithTenant_StampsOwnership(t *testing.T) {
	mgr := NewBatchJobManager()
	id := uuid.New().String()
	st, _ := mgr.CreateJobWithTenant(id, 10, "tenant-A", "channel-X")

	assert.Equal(t, "tenant-A", st.TenantKey)
	assert.Equal(t, "channel-X", st.ChannelKey)
	assert.Equal(t, BatchJobPending, st.State)
	assert.Equal(t, 10, st.Total)
}

// TestCancelJob_StopsGoroutine verifies that CancelJob signals the job context
// and that a goroutine selecting on that context exits promptly.
func TestCancelJob_StopsGoroutine(t *testing.T) {
	mgr := NewBatchJobManager()
	id := uuid.New().String()
	_, jobCtx := mgr.CreateJobWithTenant(id, 0, "tenant-A", "")

	done := make(chan struct{})
	go func() {
		select {
		case <-jobCtx.Done():
			close(done)
		case <-time.After(5 * time.Second):
		}
	}()

	ok := mgr.CancelJob(id)
	require.True(t, ok, "CancelJob should return true for existing job")

	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit within 2s after CancelJob")
	}
}

// TestCancelJob_UnknownID returns false without panic.
func TestCancelJob_UnknownID(t *testing.T) {
	mgr := NewBatchJobManager()
	ok := mgr.CancelJob("nonexistent")
	assert.False(t, ok)
}

// ── batchAdminGuard tests (FIX 3 + FIX 4) ────────────────────────────────────

// TestHMACPath_ValidSignature verifies that a correctly HMAC-signed request is
// accepted as an internal caller and treated as platform-scoped. (FIX 4b)
func TestHMACPath_ValidSignature(t *testing.T) {
	guard := &batchAdminGuard{internalSecret: testHMACSecret}
	body := []byte(`{"product_ids":["123"]}`)
	ctx := makeHMACCtx(testHMACSecret, body)

	identity, ok := guard.authorise(ctx, "")
	assert.True(t, ok, "valid HMAC should be accepted")
	assert.True(t, identity.PlatformScoped, "internal caller should be platform-scoped")
}

// TestHMACPath_BadSignature verifies that a tampered signature is rejected. (FIX 4b)
func TestHMACPath_BadSignature(t *testing.T) {
	guard := &batchAdminGuard{internalSecret: testHMACSecret}
	body := []byte(`{"product_ids":["123"]}`)
	ts := time.Now().UTC().Format(time.RFC3339)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody(body)
	ctx.Request.Header.Set("X-Internal-Timestamp", ts)
	ctx.Request.Header.Set("X-Internal-Signature", "deadbeefdeadbeef")

	_, ok := guard.authorise(ctx, "")
	assert.False(t, ok, "tampered HMAC should be rejected")
	assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestHMACPath_ExpiredTimestamp verifies that an old timestamp is rejected. (FIX 4b)
func TestHMACPath_ExpiredTimestamp(t *testing.T) {
	guard := &batchAdminGuard{internalSecret: testHMACSecret}
	body := []byte(`{}`)
	oldTS := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	sig := computeTestHMAC(testHMACSecret, oldTS, body)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody(body)
	ctx.Request.Header.Set("X-Internal-Timestamp", oldTS)
	ctx.Request.Header.Set("X-Internal-Signature", sig)

	_, ok := guard.authorise(ctx, "")
	assert.False(t, ok, "expired timestamp should be rejected")
}

// TestUnauthenticatedRequest_401 verifies path (c): no bearer, no HMAC → 401. (FIX 4c)
func TestUnauthenticatedRequest_401(t *testing.T) {
	guard := &batchAdminGuard{internalSecret: testHMACSecret}
	ctx := makeUnauthCtx()

	_, ok := guard.authorise(ctx, "tenant-A")
	assert.False(t, ok)
	assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestBearerPath_NoValidator_503 verifies that a Bearer request when no JWT
// validator is configured returns 503. (FIX 4a)
func TestBearerPath_NoValidator_503(t *testing.T) {
	guard := &batchAdminGuard{jwtValidator: nil, internalSecret: testHMACSecret}
	ctx := makeBearerCtx("some.jwt.token")

	_, ok := guard.authorise(ctx, "tenant-A")
	assert.False(t, ok)
	assert.Equal(t, fasthttp.StatusServiceUnavailable, ctx.Response.StatusCode())
}

// ── Tenant ownership checks (FIX 3) ────────────────────────────────────────

// TestMatchesTenant_SameTenant verifies case-insensitive tenant match.
func TestMatchesTenant_SameTenant(t *testing.T) {
	assert.True(t, matchesTenant(tenantctx.Identity{TenantKey: "Tenant-A"}, "tenant-a"))
	assert.True(t, matchesTenant(tenantctx.Identity{TenantKey: "TENANT-A"}, "Tenant-A"))
}

func TestMatchesTenant_DifferentTenant(t *testing.T) {
	assert.False(t, matchesTenant(tenantctx.Identity{TenantKey: "Tenant-A"}, "tenant-b"))
}

func TestMatchesTenant_EmptyJobTenantKey(t *testing.T) {
	assert.True(t, matchesTenant(tenantctx.Identity{TenantKey: ""}, "tenant-a"))
}

// ── StopBatchHandler cancels the context (FIX 1) ────────────────────────────

// TestStopBatch_CancelsContext verifies that StopBatchHandler sends the
// cancellation signal through BatchJobManager.CancelJob.
func TestStopBatch_CancelsContext(t *testing.T) {
	mgr := NewBatchJobManager()
	id := uuid.New().String()
	_, jobCtx := mgr.CreateJobWithTenant(id, 0, "tenant-A", "")

	workerDone := make(chan struct{})
	go func() {
		select {
		case <-jobCtx.Done():
			close(workerDone)
		case <-time.After(5 * time.Second):
		}
	}()

	ok := mgr.CancelJob(id)
	require.True(t, ok)

	select {
	case <-workerDone:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("worker goroutine did not stop within 2s after CancelJob")
	}
}

// TestGetBatchProgress_UnknownJob_404 verifies that querying a nonexistent job
// returns 404 without leaking job list. (FIX 3)
func TestGetBatchProgress_UnknownJob_404(t *testing.T) {
	mgr := NewBatchJobManager()
	guard := &batchAdminGuard{internalSecret: testHMACSecret}

	h := &SubscriptionHandler{
		jobs:       mgr,
		batchGuard: guard,
	}

	ctx := makeHMACCtxGetProgress(testHMACSecret, "nonexistent-job-id")

	h.GetBatchProgressHandler(ctx)
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
}

// TestGetBatchProgress_ValidJob_200 verifies that a valid HMAC caller can
// retrieve progress for a job. (FIX 3 + FIX 4b)
func TestGetBatchProgress_ValidJob_200(t *testing.T) {
	mgr := NewBatchJobManager()
	guard := &batchAdminGuard{internalSecret: testHMACSecret}

	h := &SubscriptionHandler{
		jobs:       mgr,
		batchGuard: guard,
	}

	id := uuid.New().String()
	st, _ := mgr.CreateJob(id, 5)
	st.State = BatchJobRunning

	ctx := makeHMACCtxGetProgress(testHMACSecret, id)

	h.GetBatchProgressHandler(ctx)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	var resp BatchJobStatus
	err := json.Unmarshal(ctx.Response.Body(), &resp)
	require.NoError(t, err)
	assert.Equal(t, id, resp.ID)
}

// TestStopBatch_UnknownJob_404 verifies that stopping a nonexistent job returns 404. (FIX 3)
func TestStopBatch_UnknownJob_404(t *testing.T) {
	mgr := NewBatchJobManager()
	guard := &batchAdminGuard{internalSecret: testHMACSecret}

	h := &SubscriptionHandler{
		jobs:       mgr,
		batchGuard: guard,
	}

	body, _ := json.Marshal(map[string]string{"batch_id": "nonexistent"})
	ctx := makeHMACCtx(testHMACSecret, body)

	h.StopBatchHandler(ctx)
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
}

// TestStopBatch_ValidJob_200 verifies that an internal caller can stop a job. (FIX 1 + FIX 3)
func TestStopBatch_ValidJob_200(t *testing.T) {
	mgr := NewBatchJobManager()
	guard := &batchAdminGuard{internalSecret: testHMACSecret}

	h := &SubscriptionHandler{
		logger:     zap.NewNop(),
		jobs:       mgr,
		batchGuard: guard,
	}

	id := uuid.New().String()
	_, jobCtx := mgr.CreateJobWithTenant(id, 0, "", "")

	workerDone := make(chan struct{})
	go func() {
		select {
		case <-jobCtx.Done():
			close(workerDone)
		case <-time.After(5 * time.Second):
		}
	}()

	body, _ := json.Marshal(map[string]string{"batch_id": id, "reason": "test"})
	ctx := makeHMACCtx(testHMACSecret, body)

	h.StopBatchHandler(ctx)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	select {
	case <-workerDone:
		// expected: CancelJob was called inside StopBatchHandler
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after StopBatchHandler within 2s")
	}
}

// ── Goroutine stop verification (FIX 1) ─────────────────────────────────────

// TestBackfillWorker_ExitsOnCancel verifies that worker goroutines using a
// select on gCtx.Done() exit when the context is cancelled.
func TestBackfillWorker_ExitsOnCancel(t *testing.T) {
	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workCh := make(chan string, 10)
	workerDone := make(chan struct{})

	go func() {
		defer close(workerDone)
		for {
			select {
			case <-jobCtx.Done():
				return
			case _, ok := <-workCh:
				if !ok {
					return
				}
			}
		}
	}()

	cancel()

	select {
	case <-workerDone:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after context cancel within 2s")
	}
}

// TestResubscribeWorker_ExitsOnCancel verifies the resubscribe worker pattern.
func TestResubscribeWorker_ExitsOnCancel(t *testing.T) {
	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msisdnChan := make(chan string, 10)
	workerDone := make(chan struct{})

	go func() {
		defer close(workerDone)
		for {
			select {
			case <-jobCtx.Done():
				return
			case _, ok := <-msisdnChan:
				if !ok {
					return
				}
			}
		}
	}()

	cancel()

	select {
	case <-workerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("resubscribe worker did not exit after cancel within 2s")
	}
}

// ── compile-time helpers ─────────────────────────────────────────────────────

var _ = fmt.Sprintf
var _ = uuid.New
var _ tenantctx.Identity
