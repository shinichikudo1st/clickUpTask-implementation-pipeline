package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

// Auth validates X-API-KEY first, then Authorization (trimmed), against apiSecret
// using constant-time comparison. Never logs the secret.
func Auth(apiSecret string) func(http.Handler) http.Handler {
	expected := strings.TrimSpace(apiSecret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.Header().Set("Content-Type", "application/json")

			credential := credentialFromRequest(request)
			if len(credential) != len(expected) || subtle.ConstantTimeCompare([]byte(credential), []byte(expected)) != 1 {
				responseWriter.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(responseWriter).Encode(models.ErrorResponse{
					Success: false,
					Data:    nil,
					Error: models.ErrorDetail{
						Code:    "UNAUTHORIZED",
						Message: "missing or invalid credentials",
					},
				})
				return
			}

			next.ServeHTTP(responseWriter, request)
		})
	}
}

func credentialFromRequest(request *http.Request) string {
	if value := strings.TrimSpace(request.Header.Get("X-API-KEY")); value != "" {
		return value
	}
	return strings.TrimSpace(request.Header.Get("Authorization"))
}
