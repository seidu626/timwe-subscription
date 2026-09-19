package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
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
		if !caps["subscription_only"] || !caps["invalid_msisdn_logging"] {
			t.Fatalf("missing capabilities: %v", caps)
		}
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
