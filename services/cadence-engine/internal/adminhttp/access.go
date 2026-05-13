package adminhttp

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
)

type access struct {
	validator      *auth0jwt.Validator
	staticToken    string
	allowedOrigins []string
}

const cadenceAdminAllowedCORSHeaders = "Content-Type, Authorization, X-Admin-Token, X-Tenant-Id, X-Tenant-Key, X-Tenant-Channel-Id, X-Channel-Id"

func newAccess() *access {
	domain := os.Getenv("ADMIN_AUTH0_DOMAIN")
	audience := os.Getenv("ADMIN_AUTH0_AUDIENCE")

	validator, err := auth0jwt.New(domain, audience)
	if err != nil {
		validator = nil
	}

	originsEnv := os.Getenv("CADENCE_ADMIN_CORS_ORIGINS")
	allowed := []string{"http://localhost:4200"}
	if strings.TrimSpace(originsEnv) != "" {
		parts := strings.Split(originsEnv, ",")
		allowed = allowed[:0]
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				allowed = append(allowed, p)
			}
		}
		if len(allowed) == 0 {
			allowed = []string{"http://localhost:4200"}
		}
	}

	return &access{
		validator:      validator,
		staticToken:    strings.TrimSpace(os.Getenv("CADENCE_ADMIN_TOKEN")),
		allowedOrigins: allowed,
	}
}

func (a *access) setCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	allowOrigin := ""
	for _, o := range a.allowedOrigins {
		if o == "*" {
			allowOrigin = "*"
			break
		}
		if o == origin {
			allowOrigin = origin
			break
		}
	}
	if allowOrigin == "" {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", cadenceAdminAllowedCORSHeaders)
	w.Header().Set("Access-Control-Max-Age", "600")
}

func (a *access) handlePreflight(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	a.setCORS(w, r)
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (a *access) require(w http.ResponseWriter, r *http.Request) bool {
	a.setCORS(w, r)

	if a.validateStaticToken(r) {
		*r = *r.WithContext(tenantctx.WithIdentity(r.Context(), tenantctx.Identity{
			PlatformScoped: true,
			ServiceID:      "cadence-admin-token",
			TrustSource:    tenantctx.TrustSourceTrustedService,
		}))
		return true
	}

	if a.validator == nil {
		http.Error(w, "Admin access not configured", http.StatusServiceUnavailable)
		return false
	}

	claims, err := a.validator.ValidateBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		// Do not log the Authorization header/token. Log only the failure reason.
		log.Printf("admin auth failed (cadence-engine): remote_addr=%s err=%v", r.RemoteAddr, err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	*r = *r.WithContext(tenantctx.WithIdentity(r.Context(), claims.Identity()))
	return true
}

func (a *access) validateStaticToken(r *http.Request) bool {
	expected := strings.TrimSpace(a.staticToken)
	if expected == "" {
		return false
	}

	candidates := []string{
		strings.TrimSpace(r.Header.Get("X-Admin-Token")),
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		candidates = append(candidates, strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")))
	}

	for _, token := range candidates {
		if token == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}
