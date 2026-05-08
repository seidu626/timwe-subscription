package auth0jwt

import (
	"encoding/json"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
)

type Claims struct {
	jwt.RegisteredClaims
	TenantID       string   `json:"tenant_id,omitempty"`
	TenantKey      string   `json:"tenant_key,omitempty"`
	OrgID          string   `json:"org_id,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	Permissions    []string `json:"permissions,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	PlatformScoped bool     `json:"-"`
}

func (c *Claims) UnmarshalJSON(data []byte) error {
	type registered struct {
		jwt.RegisteredClaims
	}
	var reg registered
	if err := json.Unmarshal(data, &reg); err != nil {
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.RegisteredClaims = reg.RegisteredClaims
	c.TenantID = firstString(raw, "tenant_id", "https://platform/tenant_id")
	c.TenantKey = firstString(raw, "tenant_key", "https://platform/tenant_key")
	c.OrgID = firstString(raw, "org_id", "organization_id", "https://platform/org_id")
	c.Roles = uniqueStrings(append(
		stringList(raw, "roles", "https://platform/roles"),
		splitScope(firstString(raw, "role", "https://platform/role"))...,
	))
	c.Permissions = uniqueStrings(append(
		stringList(raw, "permissions", "https://platform/permissions"),
		splitScope(firstString(raw, "scope"))...,
	))
	c.Scope = firstString(raw, "scope")
	c.PlatformScoped = tenantctx.PlatformScoped(c.Roles, c.Permissions)
	return nil
}

func (c Claims) Identity() tenantctx.Identity {
	return tenantctx.Identity{
		TenantID:       c.TenantID,
		TenantKey:      c.TenantKey,
		OrgID:          c.OrgID,
		Subject:        c.Subject,
		Roles:          append([]string(nil), c.Roles...),
		Permissions:    append([]string(nil), c.Permissions...),
		PlatformScoped: c.PlatformScoped,
		TrustSource:    tenantctx.TrustSourceJWT,
	}
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		if s, ok := value.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func stringList(raw map[string]any, keys ...string) []string {
	var out []string
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			out = append(out, splitScope(v)...)
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					out = append(out, strings.TrimSpace(s))
				}
			}
		}
	}
	return uniqueStrings(out)
}

func splitScope(scope string) []string {
	fields := strings.Fields(scope)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			out = append(out, field)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
