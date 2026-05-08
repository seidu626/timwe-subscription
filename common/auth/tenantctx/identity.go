package tenantctx

import (
	"context"
	"strings"
)

type TrustSource string

const (
	TrustSourceJWT            TrustSource = "jwt"
	TrustSourceTrustedService TrustSource = "trusted_service"
)

const FastHTTPUserValueKey = "tenantctx.identity"

type Identity struct {
	TenantID       string
	TenantKey      string
	OrgID          string
	Subject        string
	Roles          []string
	Permissions    []string
	PlatformScoped bool
	ServiceID      string
	TrustSource    TrustSource
}

func (i Identity) HasRole(role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	for _, candidate := range i.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func (i Identity) HasPermission(permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, candidate := range i.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func (i Identity) HasTenant() bool {
	return strings.TrimSpace(i.TenantID) != "" || strings.TrimSpace(i.TenantKey) != ""
}

type contextKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

func FromContext(ctx context.Context) (Identity, bool) {
	if ctx == nil {
		return Identity{}, false
	}
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}

func PlatformScoped(roles, permissions []string) bool {
	for _, role := range roles {
		switch strings.TrimSpace(role) {
		case "platform_operator", "platform_admin", "super_admin":
			return true
		}
	}
	for _, permission := range permissions {
		switch strings.TrimSpace(permission) {
		case "platform:all_tenants", "tenants:*", "admin:platform":
			return true
		}
	}
	return false
}
