package adminhttp

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
)

type access struct {
	validator      *auth0jwt.Validator
	allowedOrigins []string
}

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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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

	if a.validator == nil {
		http.Error(w, "Admin access not configured", http.StatusServiceUnavailable)
		return false
	}

	if _, err := a.validator.ValidateBearer(r.Context(), r.Header.Get("Authorization")); err != nil {
		// Do not log the Authorization header/token. Log only the failure reason.
		log.Printf("admin auth failed (cadence-engine): remote_addr=%s err=%v", r.RemoteAddr, err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}
