package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/internal/safelog"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
	"github.com/go-chi/chi/v5/middleware"
)

const maxPanicLogLen = 512

type headerTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

func (h *headerTracker) WriteHeader(statusCode int) {
	if !h.wroteHeader {
		h.wroteHeader = true
	}
	h.ResponseWriter.WriteHeader(statusCode)
}

func (h *headerTracker) Write(body []byte) (int, error) {
	if !h.wroteHeader {
		h.WriteHeader(http.StatusOK)
	}
	return h.ResponseWriter.Write(body)
}

// Recovery recovers from panics, logs a structured JSON line (including stack),
// and returns a generic 500 JSON error envelope when no response headers were sent yet.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		tracker := &headerTracker{ResponseWriter: responseWriter}

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			requestID := middleware.GetReqID(request.Context())
			if requestID == "" {
				requestID = "unknown"
			}

			panicStr := strings.TrimSpace(fmtAny(recovered))
			if len(panicStr) > maxPanicLogLen {
				panicStr = panicStr[:maxPanicLogLen] + "…"
			}
			panicStr = safelog.Redact(panicStr)
			stack := safelog.Redact(string(debug.Stack()))

			logPayload := map[string]interface{}{
				"level":      "error",
				"event":      "panic",
				"request_id": requestID,
				"method":     request.Method,
				"path":       request.URL.Path,
				"panic":      panicStr,
				"stack":      stack,
			}
			encoded, err := json.Marshal(logPayload)
			if err != nil {
				log.Printf(`{"level":"error","event":"panic_log_marshal_failed","request_id":"%s"}`, requestID)
			} else {
				log.Println(string(encoded))
			}

			if tracker.wroteHeader {
				return
			}
			tracker.ResponseWriter.Header().Set("Content-Type", "application/json")
			tracker.ResponseWriter.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(tracker.ResponseWriter).Encode(models.ErrorResponse{
				Success: false,
				Data:    nil,
				Error: models.ErrorDetail{
					Code:    "INTERNAL_ERROR",
					Message: "internal server error",
				},
			})
		}()

		next.ServeHTTP(tracker, request)
	})
}

func fmtAny(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case error:
		return t.Error()
	default:
		return fmt.Sprint(t)
	}
}
