package observability

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestHTTPMiddlewareLogsSafeTenantContextWithoutPII(t *testing.T) {
	logger, out := bufferedLogger()
	handler := HTTPMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*r = *r.WithContext(tenantctx.WithIdentity(r.Context(), tenantctx.Identity{
			TenantID:    "tenant-a",
			TrustSource: tenantctx.TrustSourceJWT,
		}))
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin?msisdn=233241234567", nil)
	req.Header.Set("X-Tenant-Channel-Id", "sms")
	req.Header.Set("X-Request-Id", "req-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	fields := decodeLogLine(t, out)
	if fields["tenant_id"] != "tenant-a" || fields["channel_id"] != "sms" || fields["request_id"] != "req-123" {
		t.Fatalf("unexpected tenant fields: %#v", fields)
	}
	if _, ok := fields["msisdn"]; ok {
		t.Fatalf("msisdn must not be logged: %#v", fields)
	}
}

func TestHTTPMiddlewareLogsUnknownTenantOnEarlyDenial(t *testing.T) {
	logger, out := bufferedLogger()
	handler := HTTPMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("X-Request-Id", "req-403")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	fields := decodeLogLine(t, out)
	if fields["tenant_id"] != UnknownLabel || fields["trust_source"] != UnknownLabel {
		t.Fatalf("expected error denial with unknown tenant fields, got %#v", fields)
	}
	if fields["request_id"] != "req-403" {
		t.Fatalf("expected request id on denial log, got %#v", fields)
	}
}

func bufferedLogger() (*zap.Logger, *bytes.Buffer) {
	out := &bytes.Buffer{}
	encoderCfg := zap.NewProductionEncoderConfig()
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(out), zap.InfoLevel)
	return zap.New(core), out
}

func decodeLogLine(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
		t.Fatalf("decode log: %v output=%s", err, out.String())
	}
	return fields
}
