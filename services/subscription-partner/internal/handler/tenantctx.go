package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/valyala/fasthttp"
)

// gatewayTenantLookup is the minimal repo interface needed by tenantRouteFromGatewayHeaders.
type gatewayTenantLookup interface {
	TenantIDByKey(tenantKey string) (string, error)
	ChannelIDByKeys(tenantID, channelKey string) (string, error)
}

// fastHTTPHeaderGetter adapts *fasthttp.RequestCtx to the tenantctx.HeaderGetter interface.
type fastHTTPHeaderGetter struct {
	ctx *fasthttp.RequestCtx
}

func (g fastHTTPHeaderGetter) Get(name string) string {
	return string(g.ctx.Request.Header.Peek(name))
}

// tenantRouteFromGatewayHeaders resolves tenant context from KrakenD-forwarded
// query params or X-Tenant-Key / X-Channel-Key headers.
//
// GatewayTrusted is set to true because partner endpoints are reachable through
// KrakenD, which rewrites public paths into backend query params. Direct-to-service
// requests that supply query params and pass the DB lookup are mitigated by nginx
// network isolation (subscription-partner is not directly internet-exposed).
//
// Error code prefixes exposed to callers:
//
//	ErrTenantKeyConflict    → 409 TENANT_KEY_CONFLICT
//	"TENANT_CONTEXT_REQUIRED" → 422
//	"UNKNOWN_TENANT"          → 422
//	"UNKNOWN_CHANNEL"         → 422
func tenantRouteFromGatewayHeaders(
	ctx *fasthttp.RequestCtx,
	repo gatewayTenantLookup,
) (domain.TenantRouteContext, error) {
	tenantKeyQuery := strings.TrimSpace(string(ctx.QueryArgs().Peek("tenant_key")))
	channelKeyQuery := strings.TrimSpace(string(ctx.QueryArgs().Peek("channel_key")))

	pair, err := tenantctx.ResolveKeyPair(
		fastHTTPHeaderGetter{ctx: ctx},
		tenantctx.KeyPair{TenantKey: tenantKeyQuery, ChannelKey: channelKeyQuery},
		tenantctx.ResolveKeyPairOptions{GatewayTrusted: true},
	)
	if err != nil {
		return domain.TenantRouteContext{}, err
	}

	tenantKey := pair.TenantKey
	channelKey := pair.ChannelKey

	if strings.TrimSpace(tenantKey) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("TENANT_CONTEXT_REQUIRED: tenant_key is required")
	}
	if strings.TrimSpace(channelKey) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("TENANT_CONTEXT_REQUIRED: channel_key is required")
	}

	tenantID, err := repo.TenantIDByKey(tenantKey)
	if err != nil || strings.TrimSpace(tenantID) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("UNKNOWN_TENANT: tenant_key %q not found", tenantKey)
	}

	channelID, err := repo.ChannelIDByKeys(tenantID, channelKey)
	if err != nil || strings.TrimSpace(channelID) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("UNKNOWN_CHANNEL: channel_key %q not found for tenant", channelKey)
	}

	return domain.TenantRouteContext{
		TenantID:   tenantID,
		TenantKey:  tenantKey,
		ChannelID:  channelID,
		ChannelKey: channelKey,
	}, nil
}

// gatewayRouteStatus maps tenantRouteFromGatewayHeaders errors to HTTP status codes.
func gatewayRouteStatus(err error) (int, string) {
	if errors.Is(err, tenantctx.ErrTenantKeyConflict) {
		return fasthttp.StatusConflict, "TENANT_KEY_CONFLICT"
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "TENANT_CONTEXT_REQUIRED"):
		return fasthttp.StatusUnprocessableEntity, "TENANT_CONTEXT_REQUIRED"
	case strings.HasPrefix(msg, "UNKNOWN_TENANT"):
		return fasthttp.StatusUnprocessableEntity, "UNKNOWN_TENANT"
	case strings.HasPrefix(msg, "UNKNOWN_CHANNEL"):
		return fasthttp.StatusUnprocessableEntity, "UNKNOWN_CHANNEL"
	default:
		return fasthttp.StatusUnprocessableEntity, "TENANT_CONTEXT_REQUIRED"
	}
}
