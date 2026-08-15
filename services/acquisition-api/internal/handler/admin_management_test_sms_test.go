package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
)

type fakeSMSTester struct {
	msisdn, tenantKey, country, message string
	err                                 error
	called                              bool
}

func (f *fakeSMSTester) SendTestMessage(msisdn, tenantKey, country, message string) error {
	f.called = true
	f.msisdn, f.tenantKey, f.country, f.message = msisdn, tenantKey, country, message
	return f.err
}

func newTestSMSCtx(body string) *fasthttp.RequestCtx {
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/sms/test")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetBodyString(body)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, revokeIdentity())
	return &ctx
}

func TestSendTestSMSSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	expectTenantLookupRevoke(mock, time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC))

	h := newRevokeTestHandler(db)
	tester := &fakeSMSTester{}
	h.SetSMSTester(tester)

	ctx := newTestSMSCtx(`{"msisdn":"0241234567","message":"ping"}`)
	h.SendTestSMS(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !tester.called || tester.tenantKey != "tenant-a" || tester.country != "GH" ||
		tester.msisdn != "0241234567" || tester.message != "ping" {
		t.Fatalf("tester got %+v", tester)
	}
	var body map[string]string
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if body["status"] != "sent" {
		t.Errorf("status = %q, want sent", body["status"])
	}
	if strings.Contains(body["msisdn"], "1234567") {
		t.Errorf("msisdn %q not masked", body["msisdn"])
	}
}

func TestSendTestSMSGatewayFailureReturns502WithReason(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	expectTenantLookupRevoke(mock, time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC))

	h := newRevokeTestHandler(db)
	h.SetSMSTester(&fakeSMSTester{err: errors.New("sms gateway returned status 403")})

	ctx := newTestSMSCtx(`{"msisdn":"0241234567"}`)
	h.SendTestSMS(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadGateway {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "403") {
		t.Errorf("body %q should carry the gateway reason", ctx.Response.Body())
	}
}

func TestSendTestSMSWithoutTesterReturns503(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	expectTenantLookupRevoke(mock, time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC))

	h := newRevokeTestHandler(db)
	ctx := newTestSMSCtx(`{"msisdn":"0241234567"}`)
	h.SendTestSMS(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestSendTestSMSMissingMSISDNReturns400(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	expectTenantLookupRevoke(mock, time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC))

	h := newRevokeTestHandler(db)
	tester := &fakeSMSTester{}
	h.SetSMSTester(tester)

	ctx := newTestSMSCtx(`{"message":"no number"}`)
	h.SendTestSMS(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if tester.called {
		t.Error("tester must not be called without an msisdn")
	}
}
