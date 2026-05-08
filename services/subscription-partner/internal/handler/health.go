package handler

import (
	"net/http"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check dependencies like database connection, cache availability, etc.
	// If everything is fine:
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
