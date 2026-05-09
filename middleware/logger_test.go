package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestRequestLogger_IncludesRequestIDMethodPathStatusLatency(t *testing.T) {
	var buf strings.Builder
	restore := captureLogOutput(&buf)
	t.Cleanup(restore)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(RequestLogger)
	router.Get("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	router.ServeHTTP(recorder, request)

	line := extractLogJSONLine(buf.String())
	if line == "" {
		t.Fatal("expected log line")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("json: %v line=%q", err, line)
	}
	if payload["request_id"] == nil || payload["request_id"] == "" {
		t.Fatalf("request_id: %v", payload["request_id"])
	}
	if payload["method"] != "GET" {
		t.Fatalf("method: %v", payload["method"])
	}
	if payload["path"] != "/v1/health" {
		t.Fatalf("path: %v", payload["path"])
	}
	if int(payload["status_code"].(float64)) != http.StatusTeapot {
		t.Fatalf("status_code: %v", payload["status_code"])
	}
	if payload["latency_ms"] == nil {
		t.Fatalf("latency_ms missing")
	}
}

func TestRequestLogger_SlowRequestUsesWarn(t *testing.T) {
	var buf strings.Builder
	restore := captureLogOutput(&buf)
	t.Cleanup(restore)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(RequestLogger)
	router.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(210 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/slow", nil))

	line := extractLogJSONLine(buf.String())
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload["level"] != "warn" || payload["event"] != "slow_request" {
		t.Fatalf("expected slow_request warn, got %+v", payload)
	}
}

func TestRequestLogger_5xxUsesErrorLevel(t *testing.T) {
	var buf strings.Builder
	restore := captureLogOutput(&buf)
	t.Cleanup(restore)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(RequestLogger)
	router.Get("/err", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/err", nil))

	line := extractLogJSONLine(buf.String())
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["level"] != "error" || payload["event"] != "internal_error" {
		t.Fatalf("got %+v", payload)
	}
}

func captureLogOutput(buf *strings.Builder) func() {
	prev := log.Writer()
	flags := log.Flags()
	prefix := log.Prefix()
	log.SetOutput(buf)
	log.SetFlags(0)
	log.SetPrefix("")
	return func() {
		log.SetOutput(prev)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	}
}

func extractLogJSONLine(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if index := strings.Index(trimmed, "{"); index >= 0 {
		return trimmed[index:]
	}
	return trimmed
}
