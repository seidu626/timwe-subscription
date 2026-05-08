package observability

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

const (
	UnknownLabel = "unknown"
	InvalidLabel = "invalid"
)

var labelValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,96}$`)

func HTTPMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		identity, _ := tenantctx.FromContext(r.Context())
		logger.Info("http request completed",
			zap.String("tenant_id", tenantLabel(identity)),
			zap.String("channel_id", SafeLabelValue(firstHeaderOrQuery(r, "X-Tenant-Channel-Id", "X-Channel-Id", "channel_id", "channelId"))),
			zap.String("trust_source", SafeLabelValue(string(identity.TrustSource))),
			zap.String("request_id", SafeLabelValue(firstHeaderOrQuery(r, "X-Request-Id", "X-Correlation-Id", "request_id", "requestId"))),
			zap.String("method", r.Method),
			zap.Int("status", rec.status),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

func SafeLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return UnknownLabel
	}
	if !labelValuePattern.MatchString(value) {
		return InvalidLabel
	}
	return value
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func tenantLabel(identity tenantctx.Identity) string {
	if identity.TenantID != "" {
		return SafeLabelValue(identity.TenantID)
	}
	return SafeLabelValue(identity.TenantKey)
}

func firstHeaderOrQuery(r *http.Request, keys ...string) string {
	if r == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	q := r.URL.Query()
	for _, key := range keys {
		if value := strings.TrimSpace(q.Get(key)); value != "" {
			return value
		}
	}
	return ""
}
