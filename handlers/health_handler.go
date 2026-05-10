package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

const serviceName = "clickup-milestone-planner-service"

const healthPingTimeout = 2 * time.Second

// HealthHandler returns 200 with service metadata. When database is non-nil,
// verifies connectivity with PingContext (503 on failure).
func HealthHandler(database *sql.DB) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		data := map[string]interface{}{
			"status":    "ok",
			"service":   serviceName,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}

		if database == nil {
			data["database"] = "not_configured"
			WriteJSONSuccess(responseWriter, http.StatusOK, data)
			return
		}

		pingCtx, cancel := context.WithTimeout(request.Context(), healthPingTimeout)
		defer cancel()
		if err := database.PingContext(pingCtx); err != nil {
			WriteJSONError(responseWriter, http.StatusServiceUnavailable, "INTERNAL_ERROR", "database unreachable")
			return
		}
		data["database"] = "connected"
		WriteJSONSuccess(responseWriter, http.StatusOK, data)
	}
}

// NotFoundHandler returns a structured 404 for unknown routes.
func NotFoundHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSONError(responseWriter, http.StatusNotFound, "NOT_FOUND", "route not found")
}

// MethodNotAllowedHandler returns a structured 405 for unsupported methods.
func MethodNotAllowedHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSONError(responseWriter, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "method not allowed")
}
