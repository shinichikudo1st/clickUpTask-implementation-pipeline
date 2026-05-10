package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

// RequireAPISecret rejects requests that do not present the configured API secret.
// Accepts Authorization: Bearer <secret> or X-API-Secret: <secret> (trimmed).
func RequireAPISecret(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg == nil || strings.TrimSpace(cfg.APISecret) == "" {
				WriteJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "API_SECRET is not configured")
				return
			}
			if !checkAPISecret(r, cfg.APISecret) {
				WriteJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or missing API secret")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func checkAPISecret(r *http.Request, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	if v := strings.TrimSpace(r.Header.Get("X-API-Secret")); v != "" {
		return constantTimeStringEqual(v, want)
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return constantTimeStringEqual(strings.TrimSpace(parts[1]), want)
	}
	return false
}

func constantTimeStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
