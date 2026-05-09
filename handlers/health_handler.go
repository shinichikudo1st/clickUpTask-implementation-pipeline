package handlers

import (
	"net/http"
	"time"
)

const serviceName = "clickup-milestone-planner-service"

// HealthHandler returns 200 with service metadata. No external dependencies.
func HealthHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSONSuccess(responseWriter, http.StatusOK, map[string]string{
		"status":    "ok",
		"service":   serviceName,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// NotFoundHandler returns a structured 404 for unknown routes.
func NotFoundHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSONError(responseWriter, http.StatusNotFound, "NOT_FOUND", "route not found")
}

// MethodNotAllowedHandler returns a structured 405 for unsupported methods.
func MethodNotAllowedHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSONError(responseWriter, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "method not allowed")
}
