package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		jobs:   NewBatchJobManager(),
	}

	// Create a known job
	st, _ := h.jobs.CreateJob("job-xyz", 50)
	h.jobs.setRunning("job-xyz")
	st.Processed = 25

	t.Run("returns progress for known job", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/api/v1/subscription-external/batch/progress?batch_id=job-xyz")
		ctx.Request.Header.SetMethod("GET")

		h.GetBatchProgressHandler(ctx)
		assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

		var resp BatchJobStatus
		_ = json.Unmarshal(ctx.Response.Body(), &resp)
		assert.Equal(t, "job-xyz", resp.ID)
		assert.Equal(t, BatchJobRunning, resp.State)
		assert.Equal(t, 50, resp.Total)
	})

	t.Run("returns 404 for unknown job", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/api/v1/subscription-external/batch/progress?batch_id=no-such-job")
		ctx.Request.Header.SetMethod("GET")

		h.GetBatchProgressHandler(ctx)
		assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	})

	t.Run("returns 400 when batch_id missing", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/api/v1/subscription-external/batch/progress")
		ctx.Request.Header.SetMethod("GET")

		h.GetBatchProgressHandler(ctx)
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	})
}

// TestStopBatchHandler verifies the stop endpoint cancels a running job.
func TestStopBatchHandler(t *testing.T) {
	h := &SubscriptionHandler{
		logger: zap.NewNop(),
		jobs:   NewBatchJobManager(),
	}

	_, _ = h.jobs.CreateJob("job-stop", 200)
	h.jobs.setRunning("job-stop")

	t.Run("stops a running job", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"batch_id": "job-stop", "reason": "test"})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")

		h.StopBatchHandler(ctx)
		assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

		// Verify state
		retrieved, _ := h.jobs.GetJob("job-stop")
		assert.Equal(t, BatchJobCancelled, retrieved.State)
	})

	t.Run("409 when job is already cancelled", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"batch_id": "job-stop"})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")

		h.StopBatchHandler(ctx)
		assert.Equal(t, fasthttp.StatusConflict, ctx.Response.StatusCode())
	})

	t.Run("404 for unknown batch", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"batch_id": "no-such"})
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetBody(body)
		ctx.Request.Header.SetMethod("POST")

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
