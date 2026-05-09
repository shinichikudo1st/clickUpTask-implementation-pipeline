package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusCapturingResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// RequestLogger emits one structured JSON log line per request with request ID,
// method, path, status, and latency. Expects chi RequestID middleware upstream.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		capturing := &statusCapturingResponseWriter{
			ResponseWriter: responseWriter,
			statusCode:     http.StatusOK,
		}

		started := time.Now()
		next.ServeHTTP(capturing, request)
		latency := time.Since(started)
		latencyMS := latency.Milliseconds()

		requestID := middleware.GetReqID(request.Context())
		if requestID == "" {
			requestID = "unknown"
		}

		stdLevel := "info"
		event := "request"
		if capturing.statusCode >= http.StatusInternalServerError {
			stdLevel = "error"
			event = "internal_error"
		} else if latency > 200*time.Millisecond {
			stdLevel = "warn"
			event = "slow_request"
		}

		payload := map[string]interface{}{
			"level":       stdLevel,
			"event":       event,
			"method":      request.Method,
			"path":        request.URL.Path,
			"status_code": capturing.statusCode,
			"latency_ms":  latencyMS,
			"request_id":  requestID,
		}
		if capturing.statusCode >= http.StatusInternalServerError {
			payload["error_code"] = "INTERNAL_ERROR"
		}

		encoded, err := json.Marshal(payload)
		if err != nil {
			log.Printf(`{"level":"error","event":"log_marshal_failed","request_id":"%s","message":%q}`, requestID, err.Error())
			return
		}
		log.Println(string(encoded))
	})
}
